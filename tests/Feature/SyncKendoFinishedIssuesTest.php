<?php

namespace Tests\Feature;

use App\Jobs\SyncKendoFinishedIssues;
use App\Models\FinishedIssue;
use App\Models\SyncedWorklog;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class SyncKendoFinishedIssuesTest extends TestCase
{
    use RefreshDatabase;

    private const ME = 4;

    private int $worklogId = 1;

    protected function setUp(): void
    {
        parent::setUp();
        config(['fylla.kendo_user_id' => self::ME]);
    }

    /** My logged time — this is how the job discovers which projects to fetch. */
    private function worklog(int $issueId, int $projectId, int $userId = self::ME, string $at = '2026-07-10 09:00:00'): void
    {
        SyncedWorklog::create([
            'kendo_worklog_id' => $this->worklogId++,
            'kendo_user_id' => $userId,
            'kendo_project_id' => $projectId,
            'kendo_issue_id' => $issueId,
            'minutes' => 60,
            'started_at' => $at,
        ]);
    }

    private function issue(int $id, int $laneId, int $assigneeId, array $overrides = []): array
    {
        return array_merge([
            'id' => $id,
            'key' => "K-$id",
            'title' => "Issue $id",
            'estimated_minutes' => 120,
            'logged_minutes' => 180,
            'lane_id' => $laneId,
            'assignee_id' => $assigneeId,
        ], $overrides);
    }

    /** Lanes with a "Done" column (id 16) plus two earlier ones. */
    private array $lanes = [
        ['id' => 13, 'title' => 'Backlog', 'order' => 1],
        ['id' => 15, 'title' => 'Continuous', 'order' => 3],
        ['id' => 16, 'title' => 'Done', 'order' => 4],
    ];

    public function test_mirrors_only_my_done_issues(): void
    {
        $this->worklog(issueId: 100, projectId: 4);
        Http::fake([
            '*/lanes' => Http::response($this->lanes),
            '*/issues' => Http::response([
                $this->issue(100, laneId: 16, assigneeId: self::ME),  // done + mine ✓
                $this->issue(101, laneId: 16, assigneeId: 99),        // done, teammate ✗
                $this->issue(102, laneId: 15, assigneeId: self::ME),  // mine, not done ✗
            ]),
        ]);

        SyncKendoFinishedIssues::dispatchSync();

        $row = FinishedIssue::sole();
        $this->assertSame(100, $row->kendo_id);
        $this->assertSame(120, $row->estimated_minutes);
        $this->assertSame(180, $row->logged_minutes);
        $this->assertSame(4, $row->project_id);
        $this->assertNotNull($row->last_worked_at); // stamped from my synced_worklogs
    }

    public function test_done_lane_falls_back_to_the_rightmost_column_without_a_done_title(): void
    {
        $this->worklog(issueId: 200, projectId: 7);
        Http::fake([
            '*/lanes' => Http::response([
                ['id' => 20, 'title' => 'To do', 'order' => 1],
                ['id' => 21, 'title' => 'Shipped', 'order' => 2], // rightmost = terminal
            ]),
            '*/issues' => Http::response([
                $this->issue(200, laneId: 21, assigneeId: self::ME),
                $this->issue(201, laneId: 20, assigneeId: self::ME),
            ]),
        ]);

        SyncKendoFinishedIssues::dispatchSync();

        $this->assertSame([200], FinishedIssue::pluck('kendo_id')->all());
    }

    public function test_issue_that_leaves_the_done_lane_is_reconciled_away(): void
    {
        $this->worklog(issueId: 300, projectId: 4);
        Http::fake([
            '*/lanes' => Http::response($this->lanes),
            '*/issues' => Http::sequence()
                ->push([$this->issue(300, laneId: 16, assigneeId: self::ME)])   // done
                ->push([$this->issue(300, laneId: 15, assigneeId: self::ME)]),  // reopened
        ]);

        SyncKendoFinishedIssues::dispatchSync();
        $this->assertSame(1, FinishedIssue::count());

        SyncKendoFinishedIssues::dispatchSync();
        $this->assertSame(0, FinishedIssue::count());
    }

    public function test_projects_i_never_logged_time_in_are_not_fetched(): void
    {
        // Only a teammate logged here → not in mine() → job makes no calls, no rows.
        $this->worklog(issueId: 400, projectId: 9, userId: 99);

        Http::fake(['*' => Http::response([], 200)]);

        SyncKendoFinishedIssues::dispatchSync();

        $this->assertSame(0, FinishedIssue::count());
        Http::assertNothingSent();
    }
}

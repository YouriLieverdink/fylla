<?php

namespace Tests\Feature;

use App\Jobs\SyncKendoProjectIssues;
use App\Models\Client;
use App\Models\Project;
use App\Models\Sprint;
use App\Models\SyncedIssue;
use App\Models\SyncedWorklog;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class SyncKendoProjectIssuesTest extends TestCase
{
    use RefreshDatabase;

    private const ME = 4;

    private int $worklogId = 1;

    protected function setUp(): void
    {
        parent::setUp();
        config(['fylla.kendo_user_id' => self::ME]);
    }

    /** My logged time — one of the two sources that put a project in the union. */
    private function worklog(int $projectId, int $userId = self::ME): void
    {
        SyncedWorklog::create([
            'kendo_worklog_id' => $this->worklogId++,
            'kendo_user_id' => $userId,
            'kendo_project_id' => $projectId,
            'kendo_issue_id' => 1,
            'minutes' => 60,
            'started_at' => '2026-07-10 09:00:00',
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
            'sprint_id' => null,
        ], $overrides);
    }

    /** Lanes: 11 first (order 1), 12 middle (order 2), 16 Done (order 3). */
    private array $lanes = [
        ['id' => 11, 'title' => 'Backlog', 'order' => 1],
        ['id' => 12, 'title' => 'In progress', 'order' => 2],
        ['id' => 16, 'title' => 'Done', 'order' => 3],
    ];

    public function test_mirrors_every_assignee_and_lane_with_position_and_name(): void
    {
        $this->worklog(projectId: 4);
        Http::fake([
            '*/lanes' => Http::response($this->lanes),
            '*/sprints' => Http::response([]),
            '*/issues' => Http::response([
                $this->issue(100, laneId: 11, assigneeId: self::ME), // first, mine
                $this->issue(101, laneId: 12, assigneeId: 99),        // middle, teammate
                $this->issue(102, laneId: 16, assigneeId: 99),        // done, teammate
            ]),
        ]);

        SyncKendoProjectIssues::dispatchSync();

        $this->assertSame(3, SyncedIssue::count()); // all assignees, all lanes
        $this->assertSame('first', SyncedIssue::firstWhere('kendo_id', 100)->lane_position);
        $this->assertSame('middle', SyncedIssue::firstWhere('kendo_id', 101)->lane_position);
        $this->assertSame('done', SyncedIssue::firstWhere('kendo_id', 102)->lane_position);
        $this->assertSame('In progress', SyncedIssue::firstWhere('kendo_id', 101)->lane_name);
        $this->assertSame(99, SyncedIssue::firstWhere('kendo_id', 101)->assignee_id);
    }

    public function test_project_set_is_the_union_of_managed_and_worked(): void
    {
        // Managed: a client project I never logged time in. Worked: a project with
        // my worklog but no client. Project 9 is neither → never fetched.
        $client = Client::create(['name' => 'Acme']);
        Project::create(['kendo_id' => 1, 'name' => 'Managed', 'client_id' => $client->id]);
        $this->worklog(projectId: 2);

        Http::fake([
            '*/projects/1/lanes' => Http::response($this->lanes),
            '*/projects/2/lanes' => Http::response($this->lanes),
            '*/sprints' => Http::response([]),
            '*/projects/1/issues' => Http::response([$this->issue(100, laneId: 16, assigneeId: self::ME)]),
            '*/projects/2/issues' => Http::response([$this->issue(200, laneId: 16, assigneeId: self::ME)]),
        ]);

        SyncKendoProjectIssues::dispatchSync();

        $this->assertSame([100, 200], SyncedIssue::orderBy('kendo_id')->pluck('kendo_id')->all());
        Http::assertNotSent(fn ($r) => str_contains($r->url(), '/projects/9/'));
    }

    public function test_lane_entered_at_stamps_on_first_sight_and_change_but_not_when_stable(): void
    {
        $this->worklog(projectId: 4);
        Http::fake([
            '*/lanes' => Http::response($this->lanes),
            '*/sprints' => Http::response([]),
            '*/issues' => Http::sequence()
                ->push([$this->issue(300, laneId: 11, assigneeId: self::ME)])  // first sight
                ->push([$this->issue(300, laneId: 11, assigneeId: self::ME)])  // unchanged lane
                ->push([$this->issue(300, laneId: 12, assigneeId: self::ME)]), // moved lane
        ]);

        $stamp = fn () => SyncedIssue::firstWhere('kendo_id', 300)->lane_entered_at?->toDateTimeString();

        $this->travelTo('2026-07-01 09:00:00');
        SyncKendoProjectIssues::dispatchSync();
        $this->assertSame('2026-07-01 09:00:00', $stamp()); // stamped on first sight

        $this->travelTo('2026-07-02 09:00:00');
        SyncKendoProjectIssues::dispatchSync();
        $this->assertSame('2026-07-01 09:00:00', $stamp()); // unchanged lane → preserved

        $this->travelTo('2026-07-03 09:00:00');
        SyncKendoProjectIssues::dispatchSync();
        $this->assertSame('2026-07-03 09:00:00', $stamp()); // lane change → re-stamped

        $this->travelBack();
    }

    public function test_issue_gone_from_the_feed_is_reconciled_away_within_the_union(): void
    {
        $this->worklog(projectId: 4);
        Http::fake([
            '*/lanes' => Http::response($this->lanes),
            '*/sprints' => Http::response([]),
            '*/issues' => Http::sequence()
                ->push([$this->issue(400, laneId: 16, assigneeId: self::ME), $this->issue(401, laneId: 12, assigneeId: 99)])
                ->push([$this->issue(400, laneId: 16, assigneeId: self::ME)]), // 401 dropped
        ]);

        SyncKendoProjectIssues::dispatchSync();
        $this->assertSame(2, SyncedIssue::count());

        SyncKendoProjectIssues::dispatchSync();
        $this->assertSame([400], SyncedIssue::pluck('kendo_id')->all());
    }

    public function test_mirrors_sprints_and_stamps_sprint_id_reconciling_stale_sprints(): void
    {
        $this->worklog(projectId: 4);
        Http::fake([
            '*/lanes' => Http::response($this->lanes),
            '*/sprints' => Http::sequence()
                ->push([['id' => 50, 'name' => 'Sprint 1', 'status' => 1, 'starts_at' => '2026-07-01', 'ends_at' => '2026-07-14']])
                ->push([['id' => 51, 'name' => 'Sprint 2', 'status' => 1, 'starts_at' => '2026-07-15', 'ends_at' => '2026-07-28']]),
            '*/issues' => Http::response([$this->issue(500, laneId: 12, assigneeId: self::ME, overrides: ['sprint_id' => 50])]),
        ]);

        SyncKendoProjectIssues::dispatchSync();
        $this->assertSame(50, SyncedIssue::firstWhere('kendo_id', 500)->sprint_id);
        $this->assertSame('Sprint 1', Sprint::firstWhere('kendo_id', 50)->name);
        $this->assertSame(1, Sprint::firstWhere('kendo_id', 50)->status);

        SyncKendoProjectIssues::dispatchSync();
        $this->assertSame([51], Sprint::pluck('kendo_id')->all()); // sprint 50 reconciled away
    }

    public function test_projects_neither_managed_nor_worked_are_never_fetched(): void
    {
        // A teammate logged in project 9, and it has no client → not in the union.
        $this->worklog(projectId: 9, userId: 99);
        Http::fake(['*' => Http::response([], 200)]);

        SyncKendoProjectIssues::dispatchSync();

        $this->assertSame(0, SyncedIssue::count());
        Http::assertNothingSent();
    }
}

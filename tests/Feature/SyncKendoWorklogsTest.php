<?php

namespace Tests\Feature;

use App\Jobs\SyncKendoWorklogs;
use App\Models\Project;
use App\Models\SyncedWorklog;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class SyncKendoWorklogsTest extends TestCase
{
    use RefreshDatabase;

    private const USER = 42;

    protected function setUp(): void
    {
        parent::setUp();
        config(['fylla.kendo_user_id' => self::USER, 'fylla.worklog_sync_days' => 90]);
    }

    /** One time-entry row in the Kendo REST shape. */
    private function entry(int $id, int $projectId, array $overrides = []): array
    {
        return array_merge([
            'id' => $id,
            'user_id' => self::USER,
            'issue_id' => 100 + $id,
            'project_id' => $projectId,
            'minutes_spent' => 60,
            'started_at' => now()->subDays(1)->toISOString(),
            'note' => "note $id",
            'issue_key' => "K-$id",
            'issue_title' => "Title $id",
        ], $overrides);
    }

    private function fakeEntries(array ...$feeds): void
    {
        $sequence = Http::sequence();
        foreach ($feeds as $feed) {
            $sequence->push(['data' => $feed], 200);
        }
        Http::fake(['*/api/time-entries*' => $sequence]);
    }

    public function test_classifies_worklog_billable_by_its_project(): void
    {
        Project::create(['kendo_id' => 1, 'name' => 'Client A', 'billable' => true]);
        Project::create(['kendo_id' => 2, 'name' => 'Internal', 'billable' => false]);

        $this->fakeEntries([$this->entry(10, 1), $this->entry(20, 2)]);
        SyncKendoWorklogs::dispatchSync();

        $this->assertTrue(SyncedWorklog::where('kendo_worklog_id', 10)->sole()->billable);
        $this->assertFalse(SyncedWorklog::where('kendo_worklog_id', 20)->sole()->billable);
        $this->assertSame([10], SyncedWorklog::billable()->pluck('kendo_worklog_id')->all());
    }

    public function test_flipping_project_billable_reclassifies_without_resync(): void
    {
        $project = Project::create(['kendo_id' => 1, 'name' => 'Client A', 'billable' => false]);
        $this->fakeEntries([$this->entry(10, 1)]);
        SyncKendoWorklogs::dispatchSync();

        $this->assertFalse(SyncedWorklog::where('kendo_worklog_id', 10)->sole()->billable);

        // Toggle the list only — no re-sync of worklogs.
        $project->update(['billable' => true]);

        $this->assertTrue(SyncedWorklog::where('kendo_worklog_id', 10)->sole()->billable);
    }

    public function test_filters_out_other_users_entries(): void
    {
        Project::create(['kendo_id' => 1, 'name' => 'Client A', 'billable' => true]);
        $this->fakeEntries([
            $this->entry(10, 1),
            $this->entry(20, 1, ['user_id' => 999]), // someone else on the admin feed
        ]);

        SyncKendoWorklogs::dispatchSync();

        $this->assertSame([10], SyncedWorklog::pluck('kendo_worklog_id')->all());
    }

    public function test_deletes_in_window_rows_absent_from_feed_but_keeps_older(): void
    {
        Project::create(['kendo_id' => 1, 'name' => 'Client A', 'billable' => true]);

        // A pre-existing row OUTSIDE the window must survive reconciliation.
        SyncedWorklog::create([
            'kendo_worklog_id' => 500,
            'kendo_project_id' => 1,
            'minutes' => 30,
            'started_at' => now()->subDays(200),
        ]);

        $this->fakeEntries(
            [$this->entry(10, 1), $this->entry(20, 1)],
            [$this->entry(10, 1)], // 20 deleted in Kendo
        );

        SyncKendoWorklogs::dispatchSync();
        SyncKendoWorklogs::dispatchSync();

        $this->assertEqualsCanonicalizing(
            [500, 10],
            SyncedWorklog::pluck('kendo_worklog_id')->all(),
        );
    }
}

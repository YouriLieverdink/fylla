<?php

namespace Tests\Feature;

use App\Models\Project;
use App\Models\SyncedWorklog;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Inertia\Testing\AssertableInertia;
use Tests\TestCase;

class UtilizationPageTest extends TestCase
{
    use RefreshDatabase;

    public function test_index_renders_breakdown_and_entries(): void
    {
        config(['fylla.kendo_user_id' => 42, 'fylla.utilization_window_weeks' => 3]);
        Project::create(['kendo_id' => 1, 'name' => 'Client A', 'billable' => true]);
        SyncedWorklog::create([
            'kendo_worklog_id' => 1, 'kendo_user_id' => 42, 'kendo_project_id' => 1, 'minutes' => 120,
            'started_at' => now()->toDateString().' 09:00:00',
            'issue_key' => 'A-1', 'issue_title' => 'Task', 'note' => 'did work',
        ]);

        $this->get('/utilization')
            ->assertOk()
            ->assertInertia(fn (AssertableInertia $page) => $page
                ->component('Utilization')
                ->has('report.weeks', 3)
                ->has('report.totals')
                ->has('entries', 1)
                ->where('entries.0.issueKey', 'A-1')
                ->where('entries.0.billable', true)
                ->where('entries.0.minutes', 120));
    }
}

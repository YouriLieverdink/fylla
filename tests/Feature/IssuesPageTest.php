<?php

namespace Tests\Feature;

use App\Jobs\SyncKendoIssues;
use App\Models\Issue;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Queue;
use Inertia\Testing\AssertableInertia;
use Tests\TestCase;

class IssuesPageTest extends TestCase
{
    use RefreshDatabase;

    public function test_index_renders_issues_from_the_local_table(): void
    {
        Issue::create([
            'kendo_id' => 1, 'key' => 'A-1', 'title' => 'Local row',
            'priority' => 'High', 'type' => 'Bug', 'synced_at' => now(),
        ]);

        $this->get('/')
            ->assertOk()
            ->assertInertia(fn (AssertableInertia $page) => $page
                ->component('Issues')
                ->has('issues', 1)
                ->where('issues.0.key', 'A-1')
                ->where('lastSyncedAt', fn ($v) => $v !== null));
    }

    public function test_sync_now_dispatches_the_job(): void
    {
        Queue::fake();

        $this->post('/sync')->assertRedirect();

        Queue::assertPushed(SyncKendoIssues::class);
    }
}

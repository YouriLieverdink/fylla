<?php

namespace Tests\Feature;

use App\Jobs\SyncKendoIssues;
use App\Models\Issue;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Cache;
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
        Cache::forever('kendo.synced_at', now()->toJSON());

        $this->get('/')
            ->assertOk()
            ->assertInertia(fn (AssertableInertia $page) => $page
                ->component('Worklist')
                ->has('items', 1)
                ->where('items.0.key', 'A-1')
                ->where('items.0.reason', 'High')
                ->where('lastSyncedAt', fn ($v) => $v !== null));
    }

    public function test_index_hides_issues_absent_from_the_latest_sync(): void
    {
        // Retained-but-done: older synced_at than the current feed.
        Issue::create([
            'kendo_id' => 1, 'key' => 'DONE-1', 'title' => 'Done, kept for history',
            'synced_at' => now()->subDay(),
        ]);
        Issue::create([
            'kendo_id' => 2, 'key' => 'OPEN-1', 'title' => 'Current work',
            'synced_at' => now(),
        ]);

        $this->get('/')
            ->assertOk()
            ->assertInertia(fn (AssertableInertia $page) => $page
                ->has('items', 1)
                ->where('items.0.key', 'OPEN-1'));
    }

    public function test_sync_now_dispatches_the_job(): void
    {
        Queue::fake();

        $this->post('/sync')->assertRedirect();

        Queue::assertPushed(SyncKendoIssues::class);
    }
}

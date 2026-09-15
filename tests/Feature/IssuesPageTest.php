<?php

namespace Tests\Feature;

use App\Jobs\SyncKendoIssues;
use App\Models\Issue;
use App\Services\TimerService;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Bus;
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

    public function test_index_reports_a_double_booked_stretch_with_both_segments(): void
    {
        Bus::fake();
        config(['fylla.display_timezone' => 'Europe/Amsterdam']);
        $this->travelTo(CarbonImmutable::parse('2026-07-13 12:00:00', 'UTC')); // 14:00 Amsterdam

        $timers = app(TimerService::class);
        $timers->start(Issue::create(['kendo_id' => 1, 'key' => 'A-1', 'title' => 'First']));
        $this->travel(60)->minutes();
        $timers->stop();                     // A ran 14:00 → 15:00, already posted

        $timers->start(Issue::create(['kendo_id' => 2, 'key' => 'B-1', 'title' => 'Second']));
        $timers->setStartTime('14:45');      // pulled 15 min into A's stretch

        $this->get('/')->assertInertia(fn (AssertableInertia $page) => $page
            ->has('overlaps', 1)
            ->where('overlaps.0.minutes', 15)
            ->where('overlaps.0.earlier.key', 'A-1')
            ->where('overlaps.0.earlier.from', '14:00')
            ->where('overlaps.0.earlier.to', '15:00')
            ->where('overlaps.0.later.key', 'B-1')
            ->where('overlaps.0.later.from', '14:45')
            ->where('overlaps.0.later.to', null));
    }

    public function test_index_reports_no_overlaps_when_nothing_is_double_booked(): void
    {
        Bus::fake();
        $timers = app(TimerService::class);
        $timers->start(Issue::create(['kendo_id' => 1, 'key' => 'A-1', 'title' => 'First']));
        $this->travel(60)->minutes();
        $timers->stop();

        $this->get('/')->assertInertia(fn (AssertableInertia $page) => $page->has('overlaps', 0));
    }
}

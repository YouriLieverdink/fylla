<?php

namespace Tests\Feature;

use App\Jobs\SyncKendoIssues;
use App\Models\Issue;
use App\Models\Timer;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Http;
use Inertia\Testing\AssertableInertia;
use Tests\TestCase;

/**
 * Ad-hoc timing (ADR-0015): search Kendo for any issue, pick it, timer starts
 * immediately. The picked issue is stored only so the timer has a subject, with
 * synced_at left unstamped so it never renders as a worklist card and is gone
 * once the timer stops.
 */
class AdhocTimingTest extends TestCase
{
    use RefreshDatabase;

    private function fakeSearch(array $rows): void
    {
        Http::fake(['*/api/issues/search*' => Http::response(['data' => $rows])]);
    }

    private function foreignRow(): array
    {
        return ['id' => 777, 'key' => 'PM-42', 'title' => 'Unassigned PM task', 'project_id' => 3];
    }

    public function test_pick_creates_unstamped_issue_and_starts_a_timer(): void
    {
        $this->fakeSearch([$this->foreignRow()]);

        $this->post('/timers/adhoc', ['key' => 'PM-42'])->assertRedirect();

        $issue = Issue::where('kendo_id', 777)->sole();
        $this->assertSame('PM-42', $issue->key);
        $this->assertSame(3, $issue->project_id);
        $this->assertNull($issue->synced_at);

        $this->assertSame(1, Timer::where('timeable_type', Issue::class)->where('timeable_id', $issue->id)->count());
    }

    public function test_picked_issue_is_absent_from_the_worklist(): void
    {
        // A normal synced issue sets the "latest sync" the worklist filters on.
        Issue::create(['kendo_id' => 1, 'key' => 'OPEN-1', 'title' => 'My work', 'synced_at' => now()]);
        $this->fakeSearch([$this->foreignRow()]);

        $this->post('/timers/adhoc', ['key' => 'PM-42']);

        $this->get('/')
            ->assertOk()
            ->assertInertia(fn (AssertableInertia $page) => $page
                ->has('items', 1)
                ->where('items.0.key', 'OPEN-1'));
    }

    public function test_sync_does_not_delete_the_picked_issue(): void
    {
        $this->fakeSearch([$this->foreignRow()]);
        $this->post('/timers/adhoc', ['key' => 'PM-42']);

        // A feed that lacks the picked issue must not delete it — it has a timer.
        Http::fake([
            '*/api/projects/*/issues' => Http::response([]),
            '*/api/issues/my' => Http::response(['data' => [], 'meta' => ['truncated' => false]]),
        ]);
        SyncKendoIssues::dispatchSync();

        $this->assertNotNull(Issue::where('kendo_id', 777)->first());
    }

    public function test_unknown_key_returns_an_error_and_starts_nothing(): void
    {
        $this->fakeSearch([]);

        $this->post('/timers/adhoc', ['key' => 'NOPE-1'])
            ->assertSessionHasErrors('adhoc');

        $this->assertSame(0, Issue::count());
        $this->assertSame(0, Timer::count());
    }
}

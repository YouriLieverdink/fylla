<?php

namespace Tests\Feature;

use App\Models\Issue;
use App\Models\Segment;
use App\Models\Timer;
use App\Models\Worklog;
use App\Services\TimerService;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class TimerServiceTest extends TestCase
{
    use RefreshDatabase;

    private TimerService $svc;

    protected function setUp(): void
    {
        parent::setUp();
        $this->svc = app(TimerService::class);
    }

    private function issue(string $key): Issue
    {
        return Issue::create([
            'kendo_id' => crc32($key), 'key' => $key, 'title' => "Issue {$key}",
        ]);
    }

    /** Count of open segments across all live timers — the invariant is ≤1. */
    private function openCount(): int
    {
        $ids = Timer::live()->pluck('id');

        return Segment::whereIn('timer_id', $ids)->whereNull('ended_at')->count();
    }

    public function test_start_pushes_and_stop_pops_with_single_open_segment(): void
    {
        $a = $this->issue('A-1');
        $b = $this->issue('B-1');

        $this->svc->start($a);
        $this->assertSame(1, Timer::live()->count());
        $this->assertSame(1, $this->openCount());

        // pushing B pauses A: still exactly one open segment, now on B
        $this->svc->start($b);
        $this->assertSame(2, Timer::live()->count());
        $this->assertSame(1, $this->openCount());
        $this->assertSame($b->id, Timer::live()->first()->issue_id);

        // stopping the top pops B and auto-resumes A
        $this->svc->stop();
        $this->assertSame(1, Timer::live()->count());
        $this->assertSame(1, $this->openCount());
        $this->assertSame($a->id, Timer::live()->first()->issue_id);

        $this->svc->stop();
        $this->assertSame(0, Timer::live()->count());
        $this->assertSame(0, $this->openCount());
    }

    public function test_start_on_a_live_issue_is_rejected(): void
    {
        $a = $this->issue('A-1');
        $this->svc->start($a);

        $this->expectException(\RuntimeException::class);
        $this->svc->start($a);
    }

    public function test_segments_sum_then_round_once_to_nearest_minute(): void
    {
        $a = $this->issue('A-1');

        // two 40s segments = 80s → sum-then-round = round(80/60) = 1 min
        // (round-then-sum would give 2 — this pins the sum-then-round rule)
        $this->svc->start($a);
        $this->travel(40)->seconds();
        $this->svc->pause();
        $this->svc->resume();
        $this->travel(40)->seconds();
        $this->svc->stop();

        $this->assertSame(1, Worklog::sole()->minutes);
    }

    public function test_session_rounding_to_zero_writes_no_worklog(): void
    {
        $a = $this->issue('A-1');

        $this->svc->start($a);
        $this->travel(20)->seconds();
        $this->svc->stop();

        $this->assertSame(0, Worklog::count());
        $this->assertSame(0, Timer::live()->count());
    }

    public function test_worklog_comment_rolls_up_nonempty_segments(): void
    {
        $a = $this->issue('A-1');

        $this->svc->start($a);
        $this->svc->comment('first');
        $this->travel(60)->seconds();
        $this->svc->pause();

        $this->svc->resume();
        $this->svc->comment('second');
        $this->travel(60)->seconds();
        $this->svc->pause();

        // third segment left without a comment — must be skipped in the rollup
        $this->svc->resume();
        $this->travel(60)->seconds();
        $this->svc->stop();

        $this->assertSame("[1/2] first\n[2/2] second", Worklog::sole()->comment);
    }

    public function test_state_survives_reload_through_the_route(): void
    {
        $a = $this->issue('A-1');

        $this->post('/timers', ['issue_id' => $a->id])->assertRedirect();
        $this->travel(120)->seconds();
        $this->post('/timers/pause')->assertRedirect();

        $this->get('/')->assertInertia(fn ($page) => $page
            ->where('timer.active.issue_id', $a->id)
            ->where('timer.active.running', false)
            ->where('timer.active.accumulated_seconds', 120));
    }
}

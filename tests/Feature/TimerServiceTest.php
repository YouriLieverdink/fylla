<?php

namespace Tests\Feature;

use App\Models\Issue;
use App\Models\Note;
use App\Models\Segment;
use App\Models\Timer;
use App\Models\Worklog;
use App\Services\TimerService;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Bus;
use Tests\TestCase;

class TimerServiceTest extends TestCase
{
    use RefreshDatabase;

    private TimerService $svc;

    protected function setUp(): void
    {
        parent::setUp();
        Bus::fake(); // isolate the state machine from the Kendo-posting side effect (#10)
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

    public function test_each_segment_posts_its_own_worklog(): void
    {
        $a = $this->issue('A-1');

        // Two segments, each rounded on its own (ADR-0005): 2 min then 3 min,
        // two Worklogs — not one summed entry.
        $this->svc->start($a);
        $this->travel(120)->seconds();
        $this->svc->pause();   // closes seg 1 → worklog
        $this->svc->resume();
        $this->travel(180)->seconds();
        $this->svc->stop();    // closes seg 2 → worklog

        $this->assertSame([2, 3], Worklog::orderBy('id')->pluck('minutes')->all());
    }

    public function test_segment_rounding_to_zero_writes_no_worklog(): void
    {
        $a = $this->issue('A-1');

        $this->svc->start($a);
        $this->travel(20)->seconds();
        $this->svc->stop();

        $this->assertSame(0, Worklog::count());
        $this->assertSame(0, Timer::live()->count());
    }

    public function test_segment_worklog_comment_joins_its_notes_in_order(): void
    {
        $a = $this->issue('A-1');

        $this->svc->start($a);
        $this->svc->addNote('first');
        $this->svc->addNote('second');
        $this->travel(60)->seconds();
        $this->svc->stop();

        $comment = Worklog::sole()->comment;
        $this->assertStringContainsString('— first', $comment);
        $this->assertStringContainsString('— second', $comment);
        $this->assertLessThan(strpos($comment, 'second'), strpos($comment, 'first'));
        // notes stamp wall-clock as HH:MM
        $this->assertMatchesRegularExpression('/^\d{2}:\d{2} — first/', $comment);
    }

    public function test_notes_attach_only_while_a_segment_is_open(): void
    {
        $a = $this->issue('A-1');

        $this->svc->start($a);
        $this->svc->pause();               // no open segment
        $this->svc->addNote('while paused');

        $this->assertSame(0, Note::count());
    }

    public function test_each_segment_keeps_its_own_notes(): void
    {
        $a = $this->issue('A-1');

        $this->svc->start($a);
        $this->svc->addNote('seg one');
        $this->travel(60)->seconds();
        $this->svc->pause();

        $this->svc->resume();
        $this->svc->addNote('seg two');
        $this->travel(60)->seconds();
        $this->svc->stop();

        [$first, $second] = Worklog::orderBy('id')->get()->all();
        $this->assertStringContainsString('seg one', $first->comment);
        $this->assertStringNotContainsString('seg two', $first->comment);
        $this->assertStringContainsString('seg two', $second->comment);
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

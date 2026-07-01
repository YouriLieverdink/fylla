<?php

namespace App\Services;

use App\Models\Issue;
use App\Models\Segment;
use App\Models\Timer;
use App\Models\Worklog;
use Illuminate\Support\Facades\DB;
use RuntimeException;

/**
 * Owns the timer-stack state machine (issue #9).
 *
 * Stack order is derived from Timer id — active = live timer (stopped_at null)
 * with the max id, the rest paused beneath in descending id (LIFO, no reorder).
 * Running ⇔ a segment is open (ended_at null); invariant: ≤1 open segment
 * across all live timers. Pause closes it, resume opens a new one.
 */
class TimerService
{
    /** Start (push) a timer for an issue. Pauses whatever ran. */
    public function start(Issue $issue): Timer
    {
        return DB::transaction(function () use ($issue) {
            if (Timer::live()->where('issue_id', $issue->id)->exists()) {
                throw new RuntimeException('Issue already has a live timer.');
            }

            $this->closeOpenSegment();

            $timer = Timer::create(['issue_id' => $issue->id]);
            $this->openSegmentOn($timer);

            return $timer;
        });
    }

    /** Pause the active timer (close its open segment). No-op if nothing runs. */
    public function pause(): void
    {
        $this->closeOpenSegment();
    }

    /** Resume the top live timer (open a fresh segment). No-op if one already runs. */
    public function resume(): void
    {
        DB::transaction(function () {
            if ($this->openSegment()) {
                return;
            }
            $top = Timer::live()->first();
            if ($top) {
                $this->openSegmentOn($top);
            }
        });
    }

    /** Stop the top live timer: roll up a worklog, then auto-resume the one beneath. */
    public function stop(): void
    {
        DB::transaction(function () {
            $top = Timer::live()->with('segments')->first();
            if (! $top) {
                return;
            }

            $this->closeOpenSegment();
            $top->refresh()->load('segments');

            $this->rollUpWorklog($top);

            $top->update(['stopped_at' => now()]);

            // auto-resume the one now on top (Q8)
            $beneath = Timer::live()->first();
            if ($beneath) {
                $this->openSegmentOn($beneath);
            }
        });
    }

    /** Patch the comment on the open (active) segment. */
    public function comment(string $comment): void
    {
        $this->openSegment()?->update(['comment' => $comment]);
    }

    /** Seconds accumulated in closed segments of a timer (excludes any open one). */
    public function accumulatedSeconds(Timer $timer): int
    {
        return (int) $timer->segments
            ->whereNotNull('ended_at')
            ->sum(fn (Segment $s) => $s->seconds());
    }

    private function openSegment(): ?Segment
    {
        $ids = Timer::live()->pluck('id');

        return Segment::whereIn('timer_id', $ids)->whereNull('ended_at')->first();
    }

    private function closeOpenSegment(): void
    {
        $this->openSegment()?->update(['ended_at' => now()]);
    }

    private function openSegmentOn(Timer $timer): Segment
    {
        return $timer->segments()->create(['started_at' => now()]);
    }

    /**
     * Sum raw seconds across all segments, round once to nearest minute (Q4).
     * Discard on 0 (Q11). Comment = non-empty segment comments joined as
     * "[i/n] …" where n = count of commented segments (Q14).
     */
    private function rollUpWorklog(Timer $timer): void
    {
        $segments = $timer->segments;
        $seconds = $segments->sum(fn (Segment $s) => $s->seconds());
        $minutes = (int) round($seconds / 60);

        if ($minutes === 0) {
            return;
        }

        $comments = $segments
            ->map(fn (Segment $s) => trim((string) $s->comment))
            ->filter()
            ->values();
        $n = $comments->count();
        $rollup = $comments
            ->map(fn (string $c, int $i) => "[".($i + 1)."/{$n}] {$c}")
            ->implode("\n");

        Worklog::create([
            'issue_id' => $timer->issue_id,
            'timer_id' => $timer->id,
            'minutes' => $minutes,
            'started_at' => $segments->min('started_at'),
            'comment' => $rollup ?: null,
        ]);
    }
}

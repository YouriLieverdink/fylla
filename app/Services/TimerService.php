<?php

namespace App\Services;

use App\Models\Issue;
use App\Models\Note;
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

    /** Stop the top live timer: close its segment (rolls up), auto-resume beneath. */
    public function stop(): void
    {
        DB::transaction(function () {
            $top = Timer::live()->first();
            if (! $top) {
                return;
            }

            $this->closeOpenSegment();
            $top->update(['stopped_at' => now()]);

            // auto-resume the one now on top (Q8)
            $beneath = Timer::live()->first();
            if ($beneath) {
                $this->openSegmentOn($beneath);
            }
        });
    }

    /** Add a note to the open (active) segment, stamped now. No-op if none is open. */
    public function addNote(string $text): void
    {
        $text = trim($text);
        $segment = $this->openSegment();
        if ($text === '' || ! $segment) {
            return;
        }
        $segment->notes()->create(['text' => $text, 'created_at' => now()]);
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

    /** Close the open segment and post its Worklog (ADR-0005). No-op if none open. */
    private function closeOpenSegment(): void
    {
        $segment = $this->openSegment();
        if (! $segment) {
            return;
        }
        $segment->update(['ended_at' => now()]);
        $this->rollUpSegment($segment);
    }

    private function openSegmentOn(Timer $timer): Segment
    {
        return $timer->segments()->create(['started_at' => now()]);
    }

    /**
     * One Worklog per segment (ADR-0005): minutes = segment seconds rounded to
     * the nearest minute, discarded on 0. Comment = the segment's notes joined
     * one "HH:MM — text" line each (wall-clock stamp), null if none.
     */
    private function rollUpSegment(Segment $segment): void
    {
        $minutes = (int) round($segment->seconds() / 60);
        if ($minutes === 0) {
            return;
        }

        $notes = $segment->notes()->orderBy('created_at')->orderBy('id')->get();
        $comment = $notes->isEmpty()
            ? null
            : $notes->map(fn (Note $n) => $n->created_at->format('H:i').' — '.$n->text)->implode("\n");

        Worklog::create([
            'issue_id' => $segment->timer->issue_id,
            'timer_id' => $segment->timer_id,
            'minutes' => $minutes,
            'started_at' => $segment->started_at,
            'comment' => $comment,
        ]);
    }
}

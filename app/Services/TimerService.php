<?php

namespace App\Services;

use App\Jobs\PostWorklog;
use App\Models\Issue;
use App\Models\Note;
use App\Models\Segment;
use App\Models\Timer;
use App\Models\Worklog;
use Illuminate\Database\Eloquent\Model;
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
    /** Start (push) a timer for a subject (Issue or PullRequest). Pauses whatever ran. */
    public function start(Model $subject): Timer
    {
        return DB::transaction(function () use ($subject) {
            $live = Timer::live()
                ->where('timeable_type', $subject->getMorphClass())
                ->where('timeable_id', $subject->getKey())
                ->exists();
            if ($live) {
                throw new RuntimeException('Already has a live timer.');
            }

            $this->closeOpenSegment();

            $timer = $subject->timers()->create();
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

    /**
     * Correct the open segment's start (a forgotten/late start). `$hm` is a
     * wall-clock "H:i" in the display tz, applied to the segment's existing
     * start date. Must be at or before now; may fall before the previous
     * (already-posted) segment — overlap is the user's to reconcile. No-op-safe
     * only when a segment is open, else throws.
     */
    public function setStartTime(string $hm): void
    {
        $segment = $this->openSegment();
        if (! $segment) {
            throw new RuntimeException('No running timer to edit.');
        }

        [$h, $m] = array_map('intval', explode(':', $hm));
        $at = $segment->started_at
            ->setTimezone(config('fylla.display_timezone'))
            ->setTime($h, $m, 0);

        if ($at->isFuture()) {
            throw new RuntimeException('Start time must be in the past.');
        }

        $segment->update(['started_at' => $at->utc()]);
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
            : $notes->map(fn (Note $n) => $n->created_at->setTimezone(config('fylla.display_timezone'))->format('H:i').' — '.$n->text)->implode("\n");

        // Stamp the Kendo coordinates from whichever subject (ADR-0009): an
        // Issue books to its own mirror fields, a PullRequest to its resolved
        // ones. issue_id is provenance — set only for Issue subjects.
        $subject = $segment->timer->timeable;
        $coords = $subject->kendoCoords();

        $worklog = Worklog::create([
            'issue_id' => $subject instanceof Issue ? $subject->id : null,
            'timer_id' => $segment->timer_id,
            'kendo_project_id' => $coords['project_id'] ?? null,
            'kendo_issue_id' => $coords['issue_id'] ?? null,
            'minutes' => $minutes,
            'started_at' => $segment->started_at,
            'comment' => $comment,
        ]);

        // Post to Kendo once the timer txn commits (#10) — afterCommit so the
        // queued job reads a committed row.
        PostWorklog::dispatch($worklog)->afterCommit();
    }
}

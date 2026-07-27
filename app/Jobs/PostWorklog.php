<?php

namespace App\Jobs;

use App\Kendo\Client as KendoClient;
use App\Models\JobRun;
use App\Models\Worklog;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;
use Throwable;

/**
 * Post one local Worklog to Kendo as a time entry (issue #10, per-segment close).
 *
 * Dispatched afterCommit from TimerService::rollUpSegment. Idempotent on
 * posted_at so a retry never double-posts. On success stamps the Kendo id;
 * on exhaustion records post_error.
 */
class PostWorklog implements ShouldQueue
{
    use Queueable;

    public int $tries = 3;

    public function __construct(private readonly Worklog $worklog) {}

    public function backoff(): array
    {
        return [10, 30];
    }

    public function handle(KendoClient $kendo): void
    {
        // Retry handle for /activity (#89). The recorder wrote this run's row on
        // JobProcessing; stamping the subject here rather than at the dispatch
        // sites means every dispatch — roll-up and retry alike — carries it.
        // $this->job is null only when handle() is called outside the queue.
        if ($this->job) {
            JobRun::where('uuid', $this->job->uuid())->update(['worklog_id' => $this->worklog->id]);
        }

        $worklog = $this->worklog->fresh();
        if ($worklog->posted_at !== null) {
            return; // already posted — no HTTP
        }

        // Coordinates are stamped on the worklog at roll-up (ADR-0009), so this
        // posts uniformly for Issue- and PR-sourced worklogs alike.
        $id = $kendo->postWorklog(
            $worklog->kendo_project_id,
            $worklog->kendo_issue_id,
            $worklog->minutes,
            $worklog->started_at->toIso8601String(),
            $worklog->comment,
        );

        $worklog->update([
            'kendo_worklog_id' => (string) $id,
            'posted_at' => now(),
            'post_error' => null,
        ]);
    }

    public function failed(Throwable $e): void
    {
        $this->worklog->update(['post_error' => $e->getMessage()]);
    }
}

<?php

namespace App\Listeners;

use App\Jobs\PostWorklog;
use App\Models\JobRun;
use Illuminate\Queue\Events\JobFailed;
use Illuminate\Queue\Events\JobProcessed;
use Illuminate\Queue\Events\JobProcessing;
use Illuminate\Support\Facades\Context;

/**
 * Records every background job run into `job_runs` (#87). Registered on the
 * queue events in AppServiceProvider, so it captures the sync (manual "Sync
 * now"), queued (scheduled) and async paths uniformly — they fire the same
 * event classes through the same dispatcher (#83). `uuid()` is the per-run key;
 * `moment_id`/`trigger` ride in via Context from the dispatch site.
 */
class JobRunRecorder
{
    /** Context keys a dispatch site sets to tag its runs; read back here. */
    public const MOMENT = 'activity_moment';

    public const TRIGGER = 'activity_trigger';

    public function processing(JobProcessing $event): void
    {
        $job = $event->job;
        $class = $job->resolveName();

        // firstOrNew (not updateOrCreate) so a retry's JobProcessing keeps the
        // original started_at — the row's creation marker.
        $run = JobRun::firstOrNew(['uuid' => $job->uuid()]);
        $run->started_at ??= now();
        $run->fill([
            'moment_id' => Context::get(self::MOMENT),
            'job_class' => $class,
            'trigger' => $this->trigger($class),
            'status' => 'running',
            'attempts' => $job->attempts(),
        ]);
        $run->save();
    }

    public function processed(JobProcessed $event): void
    {
        JobRun::where('uuid', $event->job->uuid())->update([
            'status' => 'ok',
            'finished_at' => now(),
        ]);
    }

    public function failed(JobFailed $event): void
    {
        JobRun::where('uuid', $event->job->uuid())->update([
            'status' => 'failed',
            'finished_at' => now(),
            'error' => $event->exception->getMessage(),
        ]);
    }

    /**
     * PostWorklog carries no dispatch context (null moment_id, #85), so its
     * trigger is fixed by class; every sync job takes the trigger minted at its
     * dispatch site, defaulting to scheduled when none was set.
     */
    private function trigger(string $class): string
    {
        return $class === PostWorklog::class
            ? 'worklog-post'
            : (Context::get(self::TRIGGER) ?? 'scheduled');
    }
}

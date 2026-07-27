<?php

namespace App\Http\Controllers;

use App\Jobs\PostWorklog;
use App\Models\JobRun;
use Illuminate\Http\RedirectResponse;
use Inertia\Inertia;
use Inertia\Response;

class ActivityController extends Controller
{
    /** Rows returned per request — the daily prune (#85, later slice) keeps this small. */
    private const LIMIT = 200;

    /**
     * Job & Sync Activity Log (#88): runs grouped by "sync moment". A scheduled
     * or manual sync fans out into many jobs sharing one moment_id; a worklog
     * post (null moment_id) stands alone as its own single-run group. Failures
     * roll up to the moment. Runs arrive newest-first, so first-seen insertion
     * order keeps the groups newest-first too.
     */
    public function index(): Response
    {
        $runs = JobRun::orderByDesc('started_at')->limit(self::LIMIT)->get();

        $moments = [];
        foreach ($runs as $r) {
            $key = $r->moment_id ?? 'solo-'.$r->id;
            $moments[$key] ??= [
                'id' => $key,
                'trigger' => $r->trigger,
                'startedAt' => $r->started_at, // newest run in the group (first seen)
                'runs' => [],
            ];
            $moments[$key]['runs'][] = [
                'id' => $r->id,
                'jobClass' => class_basename($r->job_class),
                'status' => $r->status,
                'startedAt' => $r->started_at,
                'finishedAt' => $r->finished_at,
                'error' => $r->error,
                'canRetry' => $r->isRetryable(),
            ];
        }

        $moments = array_map(function (array $m) {
            $statuses = array_column($m['runs'], 'status');
            $m['failedCount'] = count(array_filter($statuses, fn ($s) => $s === 'failed'));
            $m['status'] = $m['failedCount'] > 0 ? 'failed'
                : (in_array('running', $statuses, true) ? 'running' : 'ok');

            return $m;
        }, array_values($moments));

        return Inertia::render('Activity', ['moments' => $moments]);
    }

    /**
     * Re-dispatch a stuck worklog post (#89, decided in #86). A fresh queued
     * job, so it writes its own `job_runs` row and the original failed one
     * stays as a record; `PostWorklog` is idempotent on `posted_at`, so a
     * worklog posted in the interim costs no provider call.
     */
    public function retry(JobRun $jobRun): RedirectResponse
    {
        abort_unless($jobRun->isRetryable(), 404);

        PostWorklog::dispatch($jobRun->worklog);

        return back();
    }
}

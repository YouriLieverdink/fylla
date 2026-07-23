<?php

use App\Jobs\SyncGithubPullRequests;
use App\Jobs\SyncKendoIssues;
use App\Jobs\SyncKendoProjectIssues;
use App\Jobs\SyncKendoProjects;
use App\Jobs\SyncKendoUsers;
use App\Jobs\SyncKendoWorklogs;
use App\Listeners\JobRunRecorder;
use Illuminate\Foundation\Inspiring;
use Illuminate\Support\Facades\Artisan;
use Illuminate\Support\Facades\Context;
use Illuminate\Support\Facades\Schedule;
use Illuminate\Support\Str;

Artisan::command('inspire', function () {
    $this->comment(Inspiring::quote());
})->purpose('Display an inspiring quote');

/**
 * Dispatch a scheduled sync "moment": all jobs in the tick share one moment_id
 * and the scheduled trigger, which ride into each queued run via Context and
 * land in `job_runs` (#87). Context is set in the scheduler process at push
 * time, so it dehydrates into each job's payload.
 */
$syncMoment = function (array $jobs): void {
    Context::add(JobRunRecorder::MOMENT, (string) Str::uuid());
    Context::add(JobRunRecorder::TRIGGER, 'scheduled');
    foreach ($jobs as $job) {
        dispatch($job);
    }
};

Schedule::call(fn () => $syncMoment([
    new SyncKendoIssues,
    new SyncKendoProjects,
    new SyncKendoWorklogs,
    new SyncGithubPullRequests,
]))->everyFifteenMinutes();

// Team issue mirror + roster are slow-changing (and the issue job depends on
// synced_worklogs) — daily is plenty. Feed the estimation loop + Client page.
Schedule::call(fn () => $syncMoment([
    new SyncKendoUsers,
    new SyncKendoProjectIssues,
]))->daily();

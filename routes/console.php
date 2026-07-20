<?php

use App\Jobs\SyncGithubPullRequests;
use App\Jobs\SyncKendoIssues;
use App\Jobs\SyncKendoProjectIssues;
use App\Jobs\SyncKendoProjects;
use App\Jobs\SyncKendoUsers;
use App\Jobs\SyncKendoWorklogs;
use Illuminate\Foundation\Inspiring;
use Illuminate\Support\Facades\Artisan;
use Illuminate\Support\Facades\Schedule;

Artisan::command('inspire', function () {
    $this->comment(Inspiring::quote());
})->purpose('Display an inspiring quote');

Schedule::job(new SyncKendoIssues)->everyFifteenMinutes();
Schedule::job(new SyncKendoProjects)->everyFifteenMinutes();
Schedule::job(new SyncKendoWorklogs)->everyFifteenMinutes();
Schedule::job(new SyncGithubPullRequests)->everyFifteenMinutes();
// Team issue mirror + roster are slow-changing (and the issue job depends on
// synced_worklogs) — daily is plenty. Feed the estimation loop + Client page.
Schedule::job(new SyncKendoUsers)->daily();
Schedule::job(new SyncKendoProjectIssues)->daily();

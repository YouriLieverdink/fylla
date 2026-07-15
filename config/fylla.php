<?php

return [
    // REQUIRED worklog filter — the admin Kendo token returns the whole team's
    // time entries, so the sync keeps only this user's rows.
    'kendo_user_id' => env('FYLLA_KENDO_USER_ID'),

    // Rolling window (days) the worklog mirror pulls and reconciles.
    'worklog_sync_days' => 90,

    // Timestamps are stored UTC (app.timezone); note stamps render in this zone.
    'display_timezone' => 'Europe/Amsterdam',

    // GitHub PR feed (ADR-0009). Each entry is a search filter; `is:pr is:open`
    // is prepended, so entries take the full search syntax (org:, author:@me, …).
    // Comma-separated in GITHUB_PR_QUERIES.
    'github_pr_queries' => array_values(array_filter(array_map(
        'trim',
        explode(',', (string) env('GITHUB_PR_QUERIES', 'org:Back-to-code review-requested:@me,org:Back-to-code assignee:@me')),
    ))),

    // Repos (owner/name) whose PRs are never shown. Filtered at sync so they
    // never enter the local mirror. Comma-separated in GITHUB_PR_EXCLUDE_REPOS.
    'github_pr_exclude_repos' => array_values(array_filter(array_map(
        'trim',
        explode(',', (string) env('GITHUB_PR_EXCLUDE_REPOS', 'Back-to-code/daymate-api,Back-to-code/daymate-app')),
    ))),

    // Personal billable utilization (issue #12). Capacity = contracted hours
    // minus logged time off; target/soft-floor drive the trend, not pass/fail.
    'contracted_hours_per_week' => 32,
    // The one weekday not worked in the 4-day, 32h week (ISO: 1=Mon … 7=Sun).
    // Time off skips it, so a full week off subtracts 4 days, not 5.
    'contracted_off_weekday' => 5, // Friday
    'utilization_window_weeks' => 13,
    'utilization_target' => 75,
    'utilization_soft_floor' => 73,
];

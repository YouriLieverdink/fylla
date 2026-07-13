<?php

return [
    // REQUIRED worklog filter — the admin Kendo token returns the whole team's
    // time entries, so the sync keeps only this user's rows.
    'kendo_user_id' => env('FYLLA_KENDO_USER_ID'),

    // Rolling window (days) the worklog mirror pulls and reconciles.
    'worklog_sync_days' => 90,

    // Personal billable utilization (issue #12). Capacity = contracted hours
    // minus logged time off; target/soft-floor drive the trend, not pass/fail.
    'contracted_hours_per_week' => 32,
    'utilization_window_weeks' => 13,
    'utilization_target' => 75,
    'utilization_soft_floor' => 73,
];

<?php

return [
    // REQUIRED worklog filter — the admin Kendo token returns the whole team's
    // time entries, so the sync keeps only this user's rows.
    'kendo_user_id' => env('FYLLA_KENDO_USER_ID'),

    // Rolling window (days) the worklog mirror pulls and reconciles.
    'worklog_sync_days' => 90,
];

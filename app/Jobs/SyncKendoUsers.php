<?php

namespace App\Jobs;

use App\Kendo\Client as KendoClient;
use App\Models\Developer;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;

/**
 * Mirror the Kendo user roster into `developers` (issue #55 / R2) so the Client
 * context page can resolve an issue's assignee_id → a name. One global call
 * (GET /api/users, ~27 rows); slow-changing, so scheduled daily.
 */
class SyncKendoUsers implements ShouldQueue
{
    use Queueable;

    public function handle(KendoClient $kendo): void
    {
        foreach ($kendo->getUsers() as $user) {
            Developer::updateOrCreate(
                ['kendo_id' => $user['id']],
                [
                    'name' => $user['name'],
                    'email' => $user['email'],
                    'active' => $user['active'],
                    'avatar_url' => $user['avatar_url'],
                ],
            );
        }
    }
}

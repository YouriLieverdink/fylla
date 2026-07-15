<?php

namespace App\Delivery;

use App\Models\Client;
use App\Models\SyncedWorklog;
use Carbon\CarbonImmutable;

/**
 * Team-aggregate monthly delivery (issue #14, CONTEXT.md → Delivered). One card
 * per client: hours logged against the client's projects this calendar month,
 * weighed against the client's monthly target.
 *
 * Reads synced_worklogs UNSCOPED (ADR-0011) — every developer's rows plus the
 * manager's own count, billable and non-billable alike. NEVER apply mine():
 * that scope belongs to the personal utilization metric, not this team read.
 */
class DeliveryReport
{
    private CarbonImmutable $now;        // in the display timezone
    private CarbonImmutable $monthStart; // UTC, matching stored started_at
    private CarbonImmutable $monthEnd;   // UTC

    public function __construct(?CarbonImmutable $now = null)
    {
        $tz = config('fylla.display_timezone');
        $this->now = ($now ?? CarbonImmutable::now())->setTimezone($tz);
        $this->monthStart = $this->now->startOfMonth()->utc();
        $this->monthEnd = $this->now->endOfMonth()->utc();
    }

    /** @return array<int,array<string,mixed>> one card per client, alphabetical. */
    public function cards(): array
    {
        return Client::with('projects:id,client_id,kendo_id')
            ->orderBy('name')
            ->get()
            ->map(fn (Client $client) => $this->card($client))
            ->all();
    }

    private function card(Client $client): array
    {
        $logs = SyncedWorklog::whereIn('kendo_project_id', $client->projects->pluck('kendo_id'))
            ->whereBetween('started_at', [$this->monthStart, $this->monthEnd])
            ->get(['minutes', 'kendo_user_id']);

        $hours = (int) round($logs->sum('minutes') / 60);
        $developers = $logs->pluck('kendo_user_id')->unique()->count();
        $projects = $client->projects->count();
        $target = $client->monthly_target_hours;

        return [
            'id' => $client->id,
            'initials' => $this->initials($client->name),
            'name' => $client->name,
            'meta' => "{$projects} projects · {$developers} developers",
            'hours' => $hours,
            'target' => $target,
            'pct' => $target ? (int) round($hours / $target * 100) : 0,
            'status' => $target ? (int) round($hours / $target * 100).'%' : '',
            'daysLeft' => $this->daysLeft(),
        ];
    }

    /** First letter of each word, up to two, uppercased. */
    private function initials(string $name): string
    {
        return collect(preg_split('/\s+/', trim($name)))
            ->take(2)
            ->map(fn (string $w) => mb_strtoupper(mb_substr($w, 0, 1)))
            ->implode('');
    }

    /** Mon–Fri days left in the current month, including today. */
    private function daysLeft(): string
    {
        $count = 0;
        for ($d = $this->now->startOfDay(); $d->lte($this->now->endOfMonth()); $d = $d->addDay()) {
            if ($d->isWeekday()) {
                $count++;
            }
        }

        return $count.' working days left';
    }
}

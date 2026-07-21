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
            ->get(['minutes', 'kendo_user_id', 'started_at']);

        $hours = (int) round($logs->sum('minutes') / 60);
        $developers = $logs->pluck('kendo_user_id')->unique()->count();
        $projects = $client->projects->count();
        $target = $client->targetForMonth($this->now);

        // Run-rate projection: delivered scaled by working days in month / elapsed.
        $tz = config('fylla.display_timezone');
        $totalWorkingDays = $this->workingDaysBetween($this->now->startOfMonth(), $this->now->endOfMonth());
        $elapsedWorkingDays = $this->workingDaysBetween($this->now->startOfMonth(), $this->now);
        $projected = $elapsedWorkingDays > 0
            ? (int) round($hours * $totalWorkingDays / $elapsedWorkingDays)
            : null;

        // Cumulative delivered hours by day-of-month, day 1 through today — the actual line.
        $minutesByDay = [];
        foreach ($logs as $log) {
            $day = $log->started_at->setTimezone($tz)->day;
            $minutesByDay[$day] = ($minutesByDay[$day] ?? 0) + $log->minutes;
        }
        $series = [];
        $cumulative = 0;
        for ($day = 1; $day <= $this->now->day; $day++) {
            $cumulative += $minutesByDay[$day] ?? 0;
            $series[] = (int) round($cumulative / 60);
        }

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
            'projected' => $projected,
            'overUnder' => $target && $projected !== null ? $projected - $target : null,
            'series' => $series,
            'today' => $this->now->day,
            'daysInMonth' => $this->now->daysInMonth,
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
        return $this->workingDaysBetween($this->now, $this->now->endOfMonth()).' working days left';
    }

    /** Mon–Fri days from $from through $to, inclusive (both in display tz). */
    private function workingDaysBetween(CarbonImmutable $from, CarbonImmutable $to): int
    {
        $count = 0;
        for ($d = $from->startOfDay(); $d->lte($to); $d = $d->addDay()) {
            if ($d->isWeekday()) {
                $count++;
            }
        }

        return $count;
    }
}

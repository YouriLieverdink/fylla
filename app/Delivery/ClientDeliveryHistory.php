<?php

namespace App\Delivery;

use App\Models\Client;
use App\Models\SyncedWorklog;
use Carbon\CarbonImmutable;

/**
 * Delivered-vs-target history for one client (issue #67): the last N completed
 * months (`fylla.delivery_history_months`) plus the current month. Delivered is
 * the same team-aggregate read as DeliveryReport — unscoped synced_worklogs on
 * the client's projects, attributed to the display-timezone month containing
 * started_at. The cumulative gap sums completed months only; the in-progress
 * month is shown but excluded (it's where the gap gets spent).
 */
class ClientDeliveryHistory
{
    private CarbonImmutable $now; // in the display timezone

    public function __construct(?CarbonImmutable $now = null)
    {
        $this->now = ($now ?? CarbonImmutable::now())->setTimezone(config('fylla.display_timezone'));
    }

    /** @return array{rows: array<int,array<string,mixed>>, gap: ?int} oldest month first. */
    public function generate(Client $client): array
    {
        $months = (int) config('fylla.delivery_history_months');
        $tz = config('fylla.display_timezone');
        $windowStart = $this->now->startOfMonth()->subMonths($months);

        $minutesByMonth = [];
        SyncedWorklog::whereIn('kendo_project_id', $client->projects->pluck('kendo_id'))
            ->whereBetween('started_at', [$windowStart->utc(), $this->now->endOfMonth()->utc()])
            ->get(['minutes', 'started_at'])
            ->each(function (SyncedWorklog $log) use (&$minutesByMonth, $tz) {
                $key = $log->started_at->setTimezone($tz)->format('Y-m');
                $minutesByMonth[$key] = ($minutesByMonth[$key] ?? 0) + $log->minutes;
            });

        $rows = [];
        $gap = null;
        for ($offset = $months; $offset >= 0; $offset--) {
            $month = $this->now->startOfMonth()->subMonths($offset);
            $delivered = (int) round(($minutesByMonth[$month->format('Y-m')] ?? 0) / 60);
            $target = $client->targetForMonth($month);
            $delta = $target !== null ? $delivered - $target : null;

            if ($offset > 0 && $delta !== null) {
                $gap = ($gap ?? 0) + $delta;
            }

            $rows[] = [
                'month' => $month->format('M Y'),
                'delivered' => $delivered,
                'target' => $target,
                'delta' => $delta,
                'current' => $offset === 0,
            ];
        }

        return ['rows' => $rows, 'gap' => $gap];
    }
}

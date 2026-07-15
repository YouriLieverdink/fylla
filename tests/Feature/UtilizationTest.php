<?php

namespace Tests\Feature;

use App\Models\CapacityAdjustment;
use App\Models\Project;
use App\Models\SyncedWorklog;
use App\Utilization\UtilizationReport;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class UtilizationTest extends TestCase
{
    use RefreshDatabase;

    // 2026-07-13 is a Monday → the current ISO week runs 07-13..07-19.
    private const CURRENT_MONDAY = '2026-07-13';

    private const USER = 42;

    protected function setUp(): void
    {
        parent::setUp();
        config([
            'fylla.kendo_user_id' => self::USER,
            'fylla.contracted_hours_per_week' => 32,
            'fylla.utilization_window_weeks' => 3,
            'fylla.utilization_target' => 75,
            'fylla.utilization_soft_floor' => 73,
        ]);
        Project::create(['kendo_id' => 1, 'name' => 'Client A', 'billable' => true]);
        Project::create(['kendo_id' => 2, 'name' => 'Internal', 'billable' => false]);
    }

    private function log(int $id, string $day, int $hours, int $projectId = 1, int $userId = self::USER): void
    {
        SyncedWorklog::create([
            'kendo_worklog_id' => $id,
            'kendo_user_id' => $userId,
            'kendo_project_id' => $projectId,
            'minutes' => $hours * 60,
            'started_at' => $day.' 09:00:00',
        ]);
    }

    public function test_cumulative_and_trend_with_a_time_off_week(): void
    {
        // Window = 3 ISO weeks ending in the current one; now = Friday, so the
        // current week's proration is complete (4/4 worked days — Friday off).
        $now = CarbonImmutable::parse('2026-07-17 17:00');

        // Week A (two weeks ago): 16h billable, capacity 32 → 50%.
        $this->log(1, '2026-06-29', 16);
        // Week B (last week): 24h billable, 8h time off → capacity 24 → 100%.
        $this->log(2, '2026-07-06', 24);
        CapacityAdjustment::create(['date' => '2026-07-08', 'type' => 'off', 'hours' => -8, 'status' => 'confirmed']);
        // Week C (current): 20h billable, capacity 32 → 62.5%.
        $this->log(3, self::CURRENT_MONDAY, 20);
        // A non-billable entry must never touch the numerator.
        $this->log(99, self::CURRENT_MONDAY, 40, projectId: 2);

        $report = (new UtilizationReport($now))->generate();

        // Cumulative = (16+24+20) / (32+24+32) = 60/88 = 68.2%. Without the
        // time-off week it would be 60/96 = 62.5%, so the 8h shrank capacity.
        $this->assertSame(68.2, $report['value']);
        $this->assertSame([
            ['label' => 'Jun 29', 'value' => 50.0, 'billableShare' => 100.0],
            ['label' => 'Jul 6', 'value' => 100.0, 'billableShare' => 100.0],
            // Week C: 20h billable of 60h worked (40h internal) → 33.3%.
            ['label' => 'Jul 13', 'value' => 62.5, 'billableShare' => 33.3],
        ], $report['points']);
        $this->assertSame(32.0, $report['week']['capacityHours']);
        $this->assertSame(20.0, $report['week']['billableHours']);
        $this->assertSame(62.5, $report['week']['value']);
        $this->assertFalse($report['onTrack']); // 68.2 < 73 soft floor
        $this->assertSame('vs. previous 3 weeks', $report['deltaCaption']);
    }

    public function test_teammate_hours_never_touch_personal_utilization(): void
    {
        // ADR-0011 invariant: the mirror holds teammates' rows for managed-client
        // projects; the mine() scope must keep them out of the personal metric.
        // Guards a scope that looks like dead code — do not weaken.
        $now = CarbonImmutable::parse('2026-07-17 17:00');
        $this->log(1, self::CURRENT_MONDAY, 24); // mine → 24/32 = 75%
        $this->log(2, self::CURRENT_MONDAY, 40, userId: 999); // teammate, same project

        $report = (new UtilizationReport($now))->generate();

        $this->assertSame(24.0, $report['week']['billableHours']);
        $this->assertSame(75.0, $report['week']['value']);
    }

    public function test_breakdown_reconciles_with_the_dashboard(): void
    {
        // Same fixture as the cumulative test above.
        $now = CarbonImmutable::parse('2026-07-17 17:00');
        $this->log(1, '2026-06-29', 16);
        $this->log(2, '2026-07-06', 24);
        CapacityAdjustment::create(['date' => '2026-07-08', 'type' => 'off', 'hours' => -8, 'status' => 'confirmed']);
        $this->log(3, self::CURRENT_MONDAY, 20);
        // Non-billable: counts toward worked, never toward billable.
        $this->log(99, self::CURRENT_MONDAY, 40, projectId: 2);

        $report = new UtilizationReport($now);
        $gen = $report->generate();
        $bd = $report->breakdown();

        // Window totals equal the headline %.
        $this->assertSame($gen['value'], $bd['totals']['utilization']);

        // Weeks are newest-first; the current week matches the dashboard week.
        $current = $bd['weeks'][0];
        $this->assertSame('Jul 13', $current['label']);
        $this->assertSame(62.5, $current['utilization']);
        $this->assertSame(32.0, $current['capacity']);
        $this->assertSame(20.0, $current['billable']);
        // Worked = 20h billable + 40h non-billable.
        $this->assertSame(60.0, $current['worked']);

        // Last week's −8 time off surfaces as a chip on that week's row.
        $this->assertSame('Jul 6', $bd['weeks'][1]['label']);
        $this->assertSame([['hours' => -8, 'count' => 1]], $bd['weeks'][1]['adjustments']);
        $this->assertSame([], $current['adjustments']);

        // Per-week utilization equals the dashboard trend (which runs oldest→newest).
        $trendVals = array_column($gen['points'], 'value');
        $weekVals = array_reverse(array_map(fn ($w) => $w['utilization'], $bd['weeks']));
        $this->assertSame($trendVals, $weekVals);
    }

    public function test_breakdown_reports_billable_share_of_worked_hours(): void
    {
        // Same fixture as above: week C = 20h billable + 40h internal = 60h worked.
        $now = CarbonImmutable::parse('2026-07-17 17:00');
        $this->log(1, '2026-06-29', 16);
        $this->log(2, '2026-07-06', 24);
        CapacityAdjustment::create(['date' => '2026-07-08', 'type' => 'off', 'hours' => -8, 'status' => 'confirmed']);
        $this->log(3, self::CURRENT_MONDAY, 20);
        $this->log(99, self::CURRENT_MONDAY, 40, projectId: 2);

        $bd = (new UtilizationReport($now))->breakdown();

        // Current week: 20 / 60 = 33.3% (all billable in A/B → 100%).
        $this->assertSame(33.3, $bd['weeks'][0]['billableShare']);
        $this->assertSame(100.0, $bd['weeks'][1]['billableShare']);
        $this->assertSame(100.0, $bd['weeks'][2]['billableShare']);
        // Totals: Σbillable 60 / Σworked 100 = 60%.
        $this->assertSame(60.0, $bd['totals']['billableShare']);
    }

    public function test_billable_share_is_null_when_no_hours_worked(): void
    {
        // No worklogs at all → every week and the totals divide by zero → null.
        $now = CarbonImmutable::parse('2026-07-17 17:00');

        $bd = (new UtilizationReport($now))->breakdown();

        $this->assertNull($bd['weeks'][0]['billableShare']);
        $this->assertNull($bd['totals']['billableShare']);
    }

    public function test_extra_day_raises_the_week_capacity(): void
    {
        // Friday → current week complete (5/5). An extra day (+8) this week
        // lifts capacity to 40; 30h billable then reads 75%, not ~94%.
        $now = CarbonImmutable::parse('2026-07-17 17:00');
        $this->log(1, self::CURRENT_MONDAY, 30);
        CapacityAdjustment::create(['date' => '2026-07-15', 'type' => 'extra', 'hours' => 8, 'status' => 'confirmed']);

        $report = (new UtilizationReport($now))->generate();

        $this->assertSame(40.0, $report['week']['capacityHours']);
        $this->assertSame(75.0, $report['week']['value']);
    }

    public function test_current_week_capacity_prorates_over_elapsed_workdays(): void
    {
        // Now = Wednesday → 3/4 worked days elapsed (Friday off) → 32 × 3/4 = 24h.
        $now = CarbonImmutable::parse('2026-07-15 12:00');
        $this->log(1, self::CURRENT_MONDAY, 20);

        $report = (new UtilizationReport($now))->generate();

        $this->assertSame(24.0, $report['week']['capacityHours']);
    }

    public function test_zero_capacity_window_returns_no_data(): void
    {
        // Now = Monday: current week prorates to 32 × 1/4 = 8h, wiped out by a
        // full day off. Prior weeks have no time off logged but also no capacity
        // used — with a 1-week window the whole window is time off.
        config(['fylla.utilization_window_weeks' => 1]);
        $now = CarbonImmutable::parse('2026-07-13 09:00');
        CapacityAdjustment::create(['date' => self::CURRENT_MONDAY, 'type' => 'off', 'hours' => -8, 'status' => 'confirmed']);

        $report = (new UtilizationReport($now))->generate();

        $this->assertNull($report['value']);
        // Zero-capacity weeks render as gaps (null value), not dropped.
        $this->assertSame([['label' => 'Jul 13', 'value' => null, 'billableShare' => null]], $report['points']);
        $this->assertNull($report['week']['value']);
    }
}

<?php

namespace Tests\Feature;

use App\Models\Project;
use App\Models\SyncedWorklog;
use App\Models\TimeOff;
use App\Utilization\UtilizationReport;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class UtilizationTest extends TestCase
{
    use RefreshDatabase;

    // 2026-07-13 is a Monday → the current ISO week runs 07-13..07-19.
    private const CURRENT_MONDAY = '2026-07-13';

    protected function setUp(): void
    {
        parent::setUp();
        config([
            'fylla.contracted_hours_per_week' => 32,
            'fylla.utilization_window_weeks' => 3,
            'fylla.utilization_target' => 75,
            'fylla.utilization_soft_floor' => 73,
        ]);
        Project::create(['kendo_id' => 1, 'name' => 'Client A', 'billable' => true]);
        Project::create(['kendo_id' => 2, 'name' => 'Internal', 'billable' => false]);
    }

    private function log(int $id, string $day, int $hours, int $projectId = 1): void
    {
        SyncedWorklog::create([
            'kendo_worklog_id' => $id,
            'kendo_project_id' => $projectId,
            'minutes' => $hours * 60,
            'started_at' => $day.' 09:00:00',
        ]);
    }

    public function test_cumulative_and_trend_with_a_time_off_week(): void
    {
        // Window = 3 ISO weeks ending in the current one; now = Friday, so the
        // current week's proration is complete (5/5 workdays).
        $now = CarbonImmutable::parse('2026-07-17 17:00');

        // Week A (two weeks ago): 16h billable, capacity 32 → 50%.
        $this->log(1, '2026-06-29', 16);
        // Week B (last week): 24h billable, 8h time off → capacity 24 → 100%.
        $this->log(2, '2026-07-06', 24);
        TimeOff::create(['date' => '2026-07-08', 'hours' => 8]);
        // Week C (current): 20h billable, capacity 32 → 62.5%.
        $this->log(3, self::CURRENT_MONDAY, 20);
        // A non-billable entry must never touch the numerator.
        $this->log(99, self::CURRENT_MONDAY, 40, projectId: 2);

        $report = (new UtilizationReport($now))->generate();

        // Cumulative = (16+24+20) / (32+24+32) = 60/88 = 68.2%. Without the
        // time-off week it would be 60/96 = 62.5%, so the 8h shrank capacity.
        $this->assertSame(68.2, $report['value']);
        $this->assertSame([
            ['label' => 'Jun 29', 'value' => 50.0],
            ['label' => 'Jul 6', 'value' => 100.0],
            ['label' => 'Jul 13', 'value' => 62.5],
        ], $report['points']);
        $this->assertSame(32.0, $report['week']['capacityHours']);
        $this->assertSame(20.0, $report['week']['billableHours']);
        $this->assertSame(62.5, $report['week']['value']);
        $this->assertFalse($report['onTrack']); // 68.2 < 73 soft floor
        $this->assertSame('vs. previous 3 weeks', $report['deltaCaption']);
    }

    public function test_current_week_capacity_prorates_over_elapsed_workdays(): void
    {
        // Now = Wednesday → 3/5 workdays elapsed → 32 × 3/5 = 19.2h capacity.
        $now = CarbonImmutable::parse('2026-07-15 12:00');
        $this->log(1, self::CURRENT_MONDAY, 20);

        $report = (new UtilizationReport($now))->generate();

        $this->assertSame(19.2, $report['week']['capacityHours']);
    }

    public function test_zero_capacity_window_returns_no_data(): void
    {
        // Now = Monday: current week prorates to 32 × 1/5 = 6.4h, wiped out by a
        // full day off. Prior weeks have no time off logged but also no capacity
        // used — with a 1-week window the whole window is time off.
        config(['fylla.utilization_window_weeks' => 1]);
        $now = CarbonImmutable::parse('2026-07-13 09:00');
        TimeOff::create(['date' => self::CURRENT_MONDAY, 'hours' => 8]);

        $report = (new UtilizationReport($now))->generate();

        $this->assertNull($report['value']);
        $this->assertSame([], $report['points']);
        $this->assertNull($report['week']['value']);
    }
}

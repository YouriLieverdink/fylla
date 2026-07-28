<?php

namespace Tests\Feature;

use App\Models\CapacityAdjustment;
use App\Models\Project;
use App\Models\SyncedWorklog;
use App\Utilization\UtilizationProjection;
use App\Utilization\UtilizationReport;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use ReflectionMethod;
use Tests\TestCase;

/**
 * "Hours needed this week" (#105), against the math contract locked in #101.
 * A 3-week window keeps the arithmetic hand-checkable; the rule is identical
 * at the real 13.
 */
class UtilizationProjectionTest extends TestCase
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
            'fylla.contracted_off_weekday' => 5,
            'fylla.utilization_window_weeks' => 3,
            'fylla.utilization_target' => 75,
            'fylla.utilization_soft_floor' => 73,
            'fylla.utilization_pace_weeks' => 4,
        ]);
        Project::create(['kendo_id' => 1, 'name' => 'Client A', 'billable' => true]);
    }

    private function log(int $id, string $day, float $hours): void
    {
        SyncedWorklog::create([
            'kendo_worklog_id' => $id,
            'kendo_user_id' => self::USER,
            'kendo_project_id' => 1,
            'minutes' => (int) round($hours * 60),
            'started_at' => $day.' 09:00:00',
        ]);
    }

    private function off(string $date, int $hours): void
    {
        CapacityAdjustment::create(['date' => $date, 'type' => $hours < 0 ? 'off' : 'extra', 'hours' => $hours, 'status' => 'confirmed']);
    }

    private function project(string $now): ?array
    {
        return (new UtilizationProjection(new UtilizationReport(CarbonImmutable::parse($now))))->payload();
    }

    public function test_needed_hours_net_out_the_window_history(): void
    {
        // Two historic weeks at 100% (32/32) carry surplus into the window, so
        // the current week needs far less than the naive target × capacity.
        $this->log(1, '2026-06-29', 32);
        $this->log(2, '2026-07-06', 32);

        $p = $this->project('2026-07-15 12:00'); // Wednesday

        // ratio(B) = (64 + B) / 96. Target 75% → B = 8h; floor 73% → 6.08h.
        $this->assertSame(8.0, $p['thisWeek']['target']['neededTotal']);
        $this->assertSame(6.25, $p['thisWeek']['floor']['neededTotal']);
        // The trap: 0.75 × 32 = 24h is what the naive formula would demand.
        $this->assertNotSame(24.0, $p['thisWeek']['target']['neededTotal']);
        // Wednesday: 24h of 32h capacity elapsed → 8h left, so 8h is doable.
        $this->assertSame(32.0, $p['thisWeek']['capacityHours']);
        $this->assertSame(8.0, $p['thisWeek']['remainingHours']);
        $this->assertTrue($p['thisWeek']['target']['feasible']);
    }

    public function test_ask_beyond_the_weeks_remaining_capacity_is_infeasible(): void
    {
        // 25h + 25h of 64h historic capacity, 10h logged so far this week.
        $this->log(1, '2026-06-29', 25);
        $this->log(2, '2026-07-06', 25);
        $this->log(3, self::CURRENT_MONDAY, 10);

        $p = $this->project('2026-07-15 12:00'); // Wednesday → 8h left

        // ratio(B) = (50 + B) / 96 → target needs 22h total, i.e. 12h more
        // against 8h of remaining capacity. Flagged, never clamped.
        $this->assertSame(10.0, $p['thisWeek']['loggedBillableHours']);
        $this->assertSame(8.0, $p['thisWeek']['remainingHours']);
        $this->assertSame(22.0, $p['thisWeek']['target']['neededTotal']);
        $this->assertSame(12.0, $p['thisWeek']['target']['neededMore']);
        $this->assertFalse($p['thisWeek']['target']['feasible']);
        $this->assertFalse($p['thisWeek']['floor']['feasible']);
    }

    public function test_already_over_target_reports_no_more_hours_and_headroom(): void
    {
        // 24h + 24h historic of 64h capacity, 30h billable already this week.
        $this->log(1, '2026-06-29', 24);
        $this->log(2, '2026-07-06', 24);
        $this->log(3, self::CURRENT_MONDAY, 30);

        $p = $this->project('2026-07-17 17:00'); // Friday → the week is done

        // Target needs 24h total (72 − 48), the floor 22.08h — both already met.
        $this->assertSame(0.0, $p['thisWeek']['target']['neededMore']);
        $this->assertSame(0.0, $p['thisWeek']['floor']['neededMore']);
        $this->assertTrue($p['thisWeek']['target']['feasible']);
        // Slack against the floor: 30 − 22.08 = 7.92h, floored to the quarter.
        $this->assertSame(7.75, $p['thisWeek']['headroomHours']);
    }

    public function test_rounding_is_conservative_in_both_directions(): void
    {
        // Capacities 32 + 32 historic and 36 this week (an extra day) → 100h in
        // the window. 58.2h historic billable, 18.5h logged this week.
        $this->log(1, '2026-06-29', 58.2);
        $this->off('2026-07-15', 4); // extra half day, current week → cap 36
        $this->log(3, self::CURRENT_MONDAY, 18.5);

        $p = $this->project('2026-07-17 17:00'); // Friday

        // Needed rounds up: floor 14.8 → 15.0, target 16.8 → 17.0.
        $this->assertSame(15.0, $p['thisWeek']['floor']['neededTotal']);
        $this->assertSame(17.0, $p['thisWeek']['target']['neededTotal']);
        // Headroom rounds down: 18.5 − 14.8 = 3.7 → 3.5.
        $this->assertSame(3.5, $p['thisWeek']['headroomHours']);
    }

    public function test_ratio_is_monotonic_non_decreasing_in_hours(): void
    {
        // The bisection precondition, asserted directly on the step function.
        $this->log(1, '2026-06-29', 20);
        $this->log(2, '2026-07-06', 12);
        $this->log(3, self::CURRENT_MONDAY, 6);

        $projection = new UtilizationProjection(new UtilizationReport(CarbonImmutable::parse('2026-07-15 12:00')));
        $projection->payload(); // loads the window sums the ratio reads

        $ratio = new ReflectionMethod($projection, 'ratio');
        $previous = -1.0;
        for ($h = 0.0; $h <= 48.0; $h += 0.5) {
            $value = $ratio->invoke($projection, $h);
            $this->assertGreaterThanOrEqual($previous, $value, "ratio dropped at h={$h}");
            $previous = $value;
        }
        // Flat above the week's own capacity: the ceiling is inside the step.
        $this->assertSame($ratio->invoke($projection, 32.0), $ratio->invoke($projection, 60.0));
    }

    public function test_a_fully_off_current_week_nulls_every_this_week_field(): void
    {
        $this->log(1, '2026-06-29', 24);
        foreach (['2026-07-13', '2026-07-14', '2026-07-15', '2026-07-16'] as $day) {
            $this->off($day, -8);
        }

        $p = $this->project('2026-07-15 12:00');

        $this->assertSame([
            'capacityHours' => null,
            'remainingHours' => null,
            'loggedBillableHours' => null,
            'floor' => null,
            'target' => null,
            'headroomHours' => null,
        ], $p['thisWeek']);
    }

    public function test_pace_reads_complete_weeks_only_and_skips_the_ones_with_no_capacity(): void
    {
        // Two complete weeks at 20h, plus 2h already logged this (partial) week
        // — which must not enter the mean, or every Monday reads as a collapse.
        config(['fylla.utilization_pace_weeks' => 2]);
        $this->log(1, '2026-06-29', 20);
        $this->log(2, '2026-07-06', 20);
        $this->log(3, self::CURRENT_MONDAY, 2);

        $p = $this->project('2026-07-15 12:00'); // Wednesday

        $this->assertSame(20.0, $p['paceHours']);
        $this->assertSame(2, $p['paceWeeks']);

        // A week off leaves both the sum and the divisor: 20h over one week,
        // not 10h over two.
        $this->off('2026-07-06', -32);
        $p = $this->project('2026-07-15 12:00');

        $this->assertSame(20.0, $p['paceHours']);
        $this->assertSame(1, $p['paceWeeks']);
    }

    public function test_no_capacity_bearing_pace_week_is_no_pace_at_all(): void
    {
        // The one trailing pace week is fully off → null, never 0 h/wk, and the
        // time-to-band search does not run.
        config(['fylla.utilization_pace_weeks' => 1]);
        $this->log(1, '2026-06-29', 24);
        $this->off('2026-07-06', -32);

        $p = $this->project('2026-07-15 12:00');

        $this->assertNull($p['paceHours']);
        $this->assertSame(0, $p['paceWeeks']);
        $this->assertSame(
            ['weeksToFloor' => null, 'weeksToTarget' => null, 'capWeeks' => 26],
            $p['timeToBand'],
        );
    }

    public function test_time_to_band_steps_the_window_forward_at_the_current_pace(): void
    {
        // Pace = last complete week = 23.5h. Step k = 1 is the end of this week:
        // (0 + 23.5 + 23.5) / 96 = 49.0%. By k = 2 the empty week has been
        // evicted: (23.5 × 3) / 96 = 73.4% — over the floor, under the target,
        // and it stays there for every later step.
        config(['fylla.utilization_pace_weeks' => 1]);
        $this->log(1, '2026-07-06', 23.5);

        $p = $this->project('2026-07-15 12:00');

        $this->assertSame(23.5, $p['paceHours']);
        $this->assertSame(2, $p['timeToBand']['weeksToFloor']);
        $this->assertNull($p['timeToBand']['weeksToTarget']); // uncrossed by k = 26
    }

    public function test_the_current_weeks_logged_hours_floor_its_contribution(): void
    {
        // Pace = 20h, but 32h is already billed this week — hours logged cannot
        // be undone by a slower pace (#101). Floored, step 1 is (20+20+32)/96 =
        // 75% and the band is reached now; at a bare 20h it would be 62.5% and
        // never cross at all.
        config(['fylla.utilization_pace_weeks' => 1]);
        $this->log(1, '2026-06-29', 20);
        $this->log(2, '2026-07-06', 20);
        $this->log(3, self::CURRENT_MONDAY, 32);

        $p = $this->project('2026-07-17 17:00'); // Friday

        $this->assertSame(20.0, $p['paceHours']);
        $this->assertSame(1, $p['timeToBand']['weeksToFloor']);
        $this->assertSame(1, $p['timeToBand']['weeksToTarget']);
    }

    public function test_a_forward_week_with_no_capacity_is_neutral_not_a_penalty(): void
    {
        // Same as above with cur+1 booked off: the off week leaves both sums,
        // so the crossing lands on the same step.
        config(['fylla.utilization_pace_weeks' => 1]);
        $this->log(1, '2026-07-06', 23.5);
        $this->off('2026-07-20', -32);

        $p = $this->project('2026-07-15 12:00');

        $this->assertSame(2, $p['timeToBand']['weeksToFloor']);
    }

    public function test_sustained_rate_skips_a_future_week_with_no_capacity(): void
    {
        config(['fylla.utilization_window_weeks' => 13]);
        for ($week = 9; $week >= 1; $week--) {
            $this->log(20 - $week, CarbonImmutable::parse(self::CURRENT_MONDAY)->subWeeks($week)->toDateString(), 24);
        }
        $this->off('2026-07-27', -32); // cur+2 is fully off

        $p = $this->project('2026-07-15 12:00');

        // At cur+3 the window is cur-9..cur+3. The off week contributes
        // neither billable nor capacity: (216 + 3h) / (288 + 96) = 75%
        // at 24h over each of the three capacity-bearing simulated weeks.
        $this->assertSame(4, $p['sustained']['horizonWeeks']);
        $this->assertSame(3, $p['sustained']['effectiveWeeks']);
        $this->assertSame(24.0, $p['sustained']['target']['hoursPerWeek']);
        $this->assertTrue($p['sustained']['target']['feasible']);
    }

    public function test_sustained_rate_reports_infeasibility_without_dropping_the_number(): void
    {
        config(['fylla.utilization_window_weeks' => 13]);

        $p = $this->project('2026-07-15 12:00');

        // Nine empty historic weeks leave too much deficit: even 32h in each
        // simulated week reaches only 30.8%. Keep the ceiling as the reported
        // best rate and flag it rather than pretending the target is reachable.
        $this->assertSame(32.0, $p['sustained']['floor']['hoursPerWeek']);
        $this->assertFalse($p['sustained']['floor']['feasible']);
        $this->assertSame(32.0, $p['sustained']['target']['hoursPerWeek']);
        $this->assertFalse($p['sustained']['target']['feasible']);
    }

    public function test_deeply_below_the_floor_degrades_all_the_way_down(): void
    {
        // 5h in the last complete week: the floor is out of reach this week even
        // at a fully billable one, and holding 5h/wk never reaches it either.
        config(['fylla.utilization_pace_weeks' => 1]);
        $this->log(1, '2026-07-06', 5);

        $p = $this->project('2026-07-15 12:00');

        $this->assertFalse($p['thisWeek']['floor']['feasible']);
        $this->assertSame(5.0, $p['paceHours']);
        $this->assertNull($p['timeToBand']['weeksToFloor']);
        $this->assertNull($p['timeToBand']['weeksToTarget']);
    }

    public function test_payload_is_null_when_no_week_in_the_window_has_capacity(): void
    {
        config(['fylla.utilization_window_weeks' => 1]);
        foreach (['2026-07-13', '2026-07-14', '2026-07-15', '2026-07-16'] as $day) {
            $this->off($day, -8);
        }

        // Null wholesale, not an object of nulls — the card is hidden entirely.
        $this->assertNull($this->project('2026-07-15 12:00'));
    }
}

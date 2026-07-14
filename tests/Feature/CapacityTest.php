<?php

namespace Tests\Feature;

use App\Models\CapacityAdjustment;
use App\Models\VacationAccrual;
use App\Vacation\VacationLedger;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class CapacityTest extends TestCase
{
    use RefreshDatabase;

    public function test_index_renders_the_capacity_page_with_ledger(): void
    {
        CapacityAdjustment::create(['date' => '2026-07-08', 'type' => 'off', 'hours' => -8, 'status' => 'confirmed']);

        $this->get('/capacity?year=2026')
            ->assertOk()
            ->assertInertia(fn ($page) => $page
                ->component('Capacity')
                ->where('year', 2026)
                ->has('adjustments', 1)
                ->has('ledger')
                ->has('overview')
                ->where('baseCapacity', 32));
    }

    public function test_time_off_range_expands_to_worked_weekdays_and_stores_negative(): void
    {
        // 2026-07-13 Mon .. 07-19 Sun → Mon–Thu (Friday is the contracted
        // off-day; Sat/Sun weekend). A full week off is 4 × −8 = −32.
        $this->post('/capacity', [
            'type' => 'off',
            'start' => '2026-07-13',
            'end' => '2026-07-19',
            'hours' => 8,
            'reason' => 'Egypte',
        ])->assertRedirect();

        $rows = CapacityAdjustment::orderBy('date')->get();
        $this->assertCount(4, $rows);
        $this->assertSame(['2026-07-13', '2026-07-14', '2026-07-15', '2026-07-16'],
            $rows->map(fn ($r) => $r->date->toDateString())->all());
        $this->assertTrue($rows->every(fn ($r) => $r->type === 'off'));
        $this->assertTrue($rows->every(fn ($r) => $r->status === 'planned')); // default
        $this->assertEquals(-32, $rows->sum('hours'));
    }

    public function test_holiday_expands_over_weekdays_and_stores_type_holiday(): void
    {
        // 2026-01-01 is a Thursday (a worked weekday); Dec 25 that year is the
        // contracted-off Friday, which expansion would skip.
        $this->post('/capacity', ['type' => 'holiday', 'start' => '2026-01-01', 'hours' => 8, 'status' => 'confirmed'])
            ->assertRedirect();

        $row = CapacityAdjustment::firstOrFail();
        $this->assertSame('holiday', $row->type);
        $this->assertSame('confirmed', $row->status);
        $this->assertEquals(-8, $row->hours);
    }

    public function test_extra_day_allows_a_weekend_and_stores_positive(): void
    {
        $this->post('/capacity', ['type' => 'extra', 'start' => '2026-07-18', 'hours' => 8])
            ->assertRedirect();

        $row = CapacityAdjustment::firstOrFail();
        $this->assertSame('extra', $row->type);
        $this->assertEquals(8, $row->hours);
    }

    public function test_decimal_hours_are_preserved(): void
    {
        $this->post('/capacity', ['type' => 'off', 'start' => '2026-07-13', 'hours' => 1.5]);

        $this->assertEquals(-1.5, CapacityAdjustment::firstOrFail()->hours);
    }

    public function test_re_adding_a_date_upserts_rather_than_duplicating(): void
    {
        $this->post('/capacity', ['type' => 'off', 'start' => '2026-07-13', 'hours' => 8]);
        $this->post('/capacity', ['type' => 'holiday', 'start' => '2026-07-13', 'hours' => 4, 'reason' => 'Half']);

        $rows = CapacityAdjustment::all();
        $this->assertCount(1, $rows);
        $this->assertSame('holiday', $rows->first()->type);
        $this->assertEquals(-4, $rows->first()->hours);
    }

    public function test_weekend_time_off_and_holiday_are_rejected(): void
    {
        $this->post('/capacity', ['type' => 'off', 'start' => '2026-07-18', 'hours' => 8])
            ->assertSessionHasErrors('start');
        $this->post('/capacity', ['type' => 'holiday', 'start' => '2026-07-18', 'hours' => 8])
            ->assertSessionHasErrors('start');
        $this->assertSame(0, CapacityAdjustment::count());
    }

    public function test_accrual_endpoint_upserts_per_year(): void
    {
        $this->post('/capacity/accrual', ['year' => 2026, 'hours' => 200])->assertRedirect();
        $this->post('/capacity/accrual', ['year' => 2026, 'hours' => 208])->assertRedirect();

        $this->assertSame(1, VacationAccrual::count());
        $this->assertEquals(208, VacationAccrual::where('year', 2026)->value('hours'));
    }

    public function test_ledger_sums_banked_and_taken_excludes_holidays_and_sick_and_carries_over(): void
    {
        VacationAccrual::create(['year' => 2025, 'hours' => 100]);
        CapacityAdjustment::create(['date' => '2025-03-03', 'type' => 'off', 'hours' => -40, 'status' => 'planned']);
        CapacityAdjustment::create(['date' => '2025-06-06', 'type' => 'extra', 'hours' => 8, 'status' => 'confirmed']);
        // A holiday and a sick day must NOT draw the ledger even though both
        // shrink capacity.
        CapacityAdjustment::create(['date' => '2025-12-24', 'type' => 'holiday', 'hours' => -8, 'status' => 'confirmed']);
        CapacityAdjustment::create(['date' => '2025-09-09', 'type' => 'sick', 'hours' => -8, 'status' => 'confirmed']);

        VacationAccrual::create(['year' => 2026, 'hours' => 50]);
        CapacityAdjustment::create(['date' => '2026-02-02', 'type' => 'off', 'hours' => -16, 'status' => 'planned']);

        $overview = (new VacationLedger())->overview(2026);

        // 2025: 0 + 100 + 8 − 40 = 68 (holiday excluded).
        $this->assertEqualsWithDelta(8.0, $overview[2025]['banked'], 0.001);
        $this->assertEqualsWithDelta(-40.0, $overview[2025]['taken'], 0.001);
        $this->assertEqualsWithDelta(68.0, $overview[2025]['balance'], 0.001);
        // Planned sub-sums: the off day is planned, the extra is confirmed.
        $this->assertEqualsWithDelta(-40.0, $overview[2025]['takenPlanned'], 0.001);
        $this->assertEqualsWithDelta(0.0, $overview[2025]['bankedPlanned'], 0.001);

        // 2026: carryover 68 + 50 + 0 − 16 = 102. Planned rows still count.
        $this->assertEqualsWithDelta(68.0, $overview[2026]['carryover'], 0.001);
        $this->assertEqualsWithDelta(102.0, $overview[2026]['balance'], 0.001);
        $this->assertEqualsWithDelta(12.75, $overview[2026]['days'], 0.001); // 102 / 8
        $this->assertEqualsWithDelta(3.19, $overview[2026]['weeks'], 0.001); // 102 / 32, 2 dp

        // Overview looks a planning year ahead of the last data year.
        $this->assertArrayHasKey(2027, $overview);
        $this->assertEqualsWithDelta(102.0, $overview[2027]['carryover'], 0.001);
    }
}

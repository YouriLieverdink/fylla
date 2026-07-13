<?php

namespace Tests\Feature;

use App\Models\CapacityAdjustment;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class CapacityTest extends TestCase
{
    use RefreshDatabase;

    public function test_index_renders_the_capacity_page(): void
    {
        CapacityAdjustment::create(['date' => '2026-07-08', 'hours' => -8, 'reason' => 'Holiday']);

        $this->get('/capacity')
            ->assertOk()
            ->assertInertia(fn ($page) => $page
                ->component('Capacity')
                ->has('adjustments', 1)
                ->where('baseCapacity', 32));
    }

    public function test_time_off_range_expands_to_worked_weekdays_and_stores_negative(): void
    {
        // 2026-07-13 Mon .. 2026-07-19 Sun → Mon–Thu only. Friday is the
        // contracted non-working day (config), Sat/Sun are weekend. A full
        // week off is 4 × −8 = −32, matching the 32h contract → 0h capacity.
        $this->post('/capacity', [
            'type' => 'off',
            'start' => '2026-07-13',
            'end' => '2026-07-19',
            'hours' => 8,
            'reason' => 'Holiday',
        ])->assertRedirect();

        $rows = CapacityAdjustment::orderBy('date')->get();
        $this->assertCount(4, $rows);
        $this->assertSame(['2026-07-13', '2026-07-14', '2026-07-15', '2026-07-16'],
            $rows->map(fn ($r) => $r->date->toDateString())->all());
        $this->assertTrue($rows->every(fn ($r) => $r->hours === -8));
        $this->assertSame(-32, $rows->sum('hours'));
    }

    public function test_extra_day_allows_a_weekend_and_stores_positive(): void
    {
        $this->post('/capacity', ['type' => 'extra', 'start' => '2026-07-18', 'hours' => 8])
            ->assertRedirect();

        $rows = CapacityAdjustment::all();
        $this->assertCount(1, $rows);
        $this->assertSame(8, $rows->first()->hours);
    }

    public function test_re_adding_a_date_upserts_rather_than_duplicating(): void
    {
        $this->post('/capacity', ['type' => 'off', 'start' => '2026-07-13', 'hours' => 8]);
        $this->post('/capacity', ['type' => 'off', 'start' => '2026-07-13', 'hours' => 4, 'reason' => 'Half day']);

        $rows = CapacityAdjustment::all();
        $this->assertCount(1, $rows);
        $this->assertSame(-4, $rows->first()->hours);
        $this->assertSame('Half day', $rows->first()->reason);
    }

    public function test_weekend_time_off_is_rejected(): void
    {
        $this->post('/capacity', ['type' => 'off', 'start' => '2026-07-18', 'hours' => 8])
            ->assertSessionHasErrors('start');
        $this->assertSame(0, CapacityAdjustment::count());
    }
}

<?php

namespace Tests\Feature;

use App\Delivery\DeliveryReport;
use App\Models\Client;
use App\Models\ClientTargetChange;
use App\Models\Project;
use App\Models\SyncedWorklog;
use App\Utilization\UtilizationReport;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class DeliveryReportTest extends TestCase
{
    use RefreshDatabase;

    private const ME = 42;

    protected function setUp(): void
    {
        parent::setUp();
        config([
            'fylla.kendo_user_id' => self::ME,
            'fylla.contracted_hours_per_week' => 32,
            'fylla.utilization_window_weeks' => 3,
            'fylla.utilization_target' => 75,
            'fylla.utilization_soft_floor' => 73,
        ]);
    }

    private function log(int $id, string $day, int $hours, int $projectKendoId, int $userId): void
    {
        SyncedWorklog::create([
            'kendo_worklog_id' => $id,
            'kendo_user_id' => $userId,
            'kendo_project_id' => $projectKendoId,
            'minutes' => $hours * 60,
            'started_at' => $day.' 09:00:00',
        ]);
    }

    public function test_delivered_is_the_unscoped_team_sum_for_the_month(): void
    {
        $now = CarbonImmutable::parse('2026-07-15 12:00', 'Europe/Amsterdam');

        $client = Client::create(['name' => 'Meridian Studio', 'monthly_target_hours' => 160]);
        Project::create(['kendo_id' => 1, 'name' => 'App', 'billable' => true, 'client_id' => $client->id]);
        Project::create(['kendo_id' => 2, 'name' => 'Admin', 'billable' => false, 'client_id' => $client->id]);
        // A project on nobody's client — must never leak into a client's delivery.
        Project::create(['kendo_id' => 9, 'name' => 'Other', 'billable' => true]);

        // Manager's own hours (billable + non-billable) + two teammates, all in July.
        $this->log(1, '2026-07-02', 10, 1, self::ME);
        $this->log(2, '2026-07-03', 5, 2, self::ME);      // non-billable still counts
        $this->log(3, '2026-07-10', 20, 1, 43);           // teammate
        $this->log(4, '2026-07-11', 15, 1, 44);           // teammate
        // Out-of-month and off-client rows that must be excluded.
        $this->log(5, '2026-06-30', 40, 1, 43);           // previous month
        $this->log(6, '2026-08-01', 40, 1, 43);           // next month
        $this->log(7, '2026-07-05', 40, 9, self::ME);     // unassigned project

        $card = (new DeliveryReport($now))->cards()[0];

        $this->assertSame('Meridian Studio', $card['name']);
        $this->assertSame('MS', $card['initials']);
        $this->assertSame(50, $card['hours']);            // 10+5+20+15
        $this->assertSame(160, $card['target']);
        $this->assertSame(31, $card['pct']);              // round(50/160*100)
        $this->assertSame('31%', $card['status']);
        $this->assertSame('2 projects · 3 developers', $card['meta']);
    }

    public function test_client_without_a_target_shows_hours_and_no_percent(): void
    {
        $now = CarbonImmutable::parse('2026-07-15 12:00', 'Europe/Amsterdam');

        $client = Client::create(['name' => 'Acme', 'monthly_target_hours' => null]);
        Project::create(['kendo_id' => 1, 'name' => 'App', 'billable' => true, 'client_id' => $client->id]);
        $this->log(1, '2026-07-02', 8, 1, self::ME);

        $card = (new DeliveryReport($now))->cards()[0];

        $this->assertSame(8, $card['hours']);
        $this->assertNull($card['target']);
        $this->assertSame('', $card['status']);
        $this->assertNull($card['overUnder']); // no target → nothing to pace against
    }

    public function test_projection_scales_delivered_by_working_days(): void
    {
        // July 2026 has 23 working days (Mon–Fri). Through the 15th, 11 have elapsed
        // (Jul 1 is a Wednesday). Under-delivering → behind the target.
        $now = CarbonImmutable::parse('2026-07-15 12:00', 'Europe/Amsterdam');

        $client = Client::create(['name' => 'Meridian Studio', 'monthly_target_hours' => 160]);
        Project::create(['kendo_id' => 1, 'name' => 'App', 'billable' => true, 'client_id' => $client->id]);
        $this->log(1, '2026-07-02', 30, 1, self::ME);
        $this->log(2, '2026-07-10', 30, 1, 43);

        $card = (new DeliveryReport($now))->cards()[0];

        $this->assertSame(60, $card['hours']);
        $this->assertSame(125, $card['projected']);   // round(60 * 23 / 11)
        $this->assertSame(-35, $card['overUnder']);    // 125 − 160, will land under
    }

    public function test_projection_over_target(): void
    {
        $now = CarbonImmutable::parse('2026-07-15 12:00', 'Europe/Amsterdam');

        $client = Client::create(['name' => 'Meridian Studio', 'monthly_target_hours' => 160]);
        Project::create(['kendo_id' => 1, 'name' => 'App', 'billable' => true, 'client_id' => $client->id]);
        $this->log(1, '2026-07-02', 100, 1, self::ME);

        $card = (new DeliveryReport($now))->cards()[0];

        $this->assertSame(100, $card['hours']);
        $this->assertSame(209, $card['projected']);   // round(100 * 23 / 11)
        $this->assertSame(49, $card['overUnder']);     // over the agreed hours
    }

    public function test_cumulative_series_buckets_by_day_of_month(): void
    {
        $now = CarbonImmutable::parse('2026-07-04 12:00', 'Europe/Amsterdam');

        $client = Client::create(['name' => 'Meridian Studio', 'monthly_target_hours' => 160]);
        Project::create(['kendo_id' => 1, 'name' => 'App', 'billable' => true, 'client_id' => $client->id]);
        $this->log(1, '2026-07-01', 4, 1, self::ME);
        $this->log(2, '2026-07-03', 6, 1, 43);

        $card = (new DeliveryReport($now))->cards()[0];

        // Day 1..4 cumulative: 4, 4, 10, 10.
        $this->assertSame([4, 4, 10, 10], $card['series']);
        $this->assertSame(4, $card['today']);
        $this->assertSame(31, $card['daysInMonth']);
    }

    public function test_override_for_the_current_month_replaces_the_default_target(): void
    {
        $now = CarbonImmutable::parse('2026-07-15 12:00', 'Europe/Amsterdam');

        $client = Client::create(['name' => 'Meridian Studio', 'monthly_target_hours' => 160]);
        Project::create(['kendo_id' => 1, 'name' => 'App', 'billable' => true, 'client_id' => $client->id]);
        ClientTargetChange::create(['client_id' => $client->id, 'effective_from' => '2026-07-01', 'hours' => 100]);
        // Effective next month — must not apply yet.
        ClientTargetChange::create(['client_id' => $client->id, 'effective_from' => '2026-08-01', 'hours' => 300]);
        $this->log(1, '2026-07-02', 50, 1, self::ME);

        $card = (new DeliveryReport($now))->cards()[0];

        $this->assertSame(100, $card['target']);
        $this->assertSame(50, $card['pct']);   // round(50/100*100)
        $this->assertSame('50%', $card['status']);
    }

    public function test_personal_utilization_ignores_teammate_rows(): void
    {
        // Regression for ADR-0011: the shared mirror holds teammates' rows, but
        // the mine()-scoped personal metric must not see them.
        $now = CarbonImmutable::parse('2026-07-17 17:00'); // Friday, current week complete

        $client = Client::create(['name' => 'Meridian Studio', 'monthly_target_hours' => 160]);
        Project::create(['kendo_id' => 1, 'name' => 'App', 'billable' => true, 'client_id' => $client->id]);

        // Me: 24h billable this week → against 32h capacity = 75%.
        $this->log(1, '2026-07-13', 24, 1, self::ME);
        // Teammate: a mountain of hours on the same project — team read sees it,
        // personal read must not.
        $this->log(2, '2026-07-13', 200, 1, 43);

        $delivery = (new DeliveryReport(CarbonImmutable::parse('2026-07-15 12:00', 'Europe/Amsterdam')))->cards()[0];
        $this->assertSame(224, $delivery['hours']); // team sum includes the teammate

        $util = (new UtilizationReport($now))->generate();
        $this->assertSame(75.0, $util['week']['value']); // personal metric unmoved
    }
}

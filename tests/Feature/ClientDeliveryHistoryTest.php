<?php

namespace Tests\Feature;

use App\Delivery\ClientDeliveryHistory;
use App\Models\Client;
use App\Models\ClientTargetChange;
use App\Models\Project;
use App\Models\SyncedWorklog;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class ClientDeliveryHistoryTest extends TestCase
{
    use RefreshDatabase;

    private const NOW = '2026-07-20 12:00:00';

    private const PID = 10;

    private int $seq = 1;

    private Client $client;

    protected function setUp(): void
    {
        parent::setUp();
        $this->client = Client::create(['name' => 'Meridian', 'monthly_target_hours' => 100]);
        Project::create(['kendo_id' => self::PID, 'name' => 'Web', 'client_id' => $this->client->id]);
    }

    private function history(): array
    {
        return (new ClientDeliveryHistory(CarbonImmutable::parse(self::NOW)))
            ->generate($this->client->fresh()->load('projects'));
    }

    private function worklog(int $minutes, string $at): void
    {
        SyncedWorklog::create([
            'kendo_worklog_id' => $this->seq++,
            'kendo_user_id' => 1,
            'kendo_project_id' => self::PID,
            'minutes' => $minutes,
            'started_at' => $at,
        ]);
    }

    public function test_delivered_per_month_reconciles_and_gap_excludes_current_month(): void
    {
        $this->worklog(600, '2026-03-15 09:00:00');  // before the window — ignored
        $this->worklog(3600, '2026-04-15 09:00:00'); // 60h
        $this->worklog(5400, '2026-05-15 09:00:00'); // 90h
        $this->worklog(7200, '2026-06-15 09:00:00'); // 120h
        $this->worklog(2400, '2026-07-10 09:00:00'); // 40h so far

        $history = $this->history();

        $this->assertSame(
            ['Apr 2026', 'May 2026', 'Jun 2026', 'Jul 2026'],
            array_column($history['rows'], 'month'),
        );
        $this->assertSame([60, 90, 120, 40], array_column($history['rows'], 'delivered'));
        $this->assertSame([-40, -10, 20, -60], array_column($history['rows'], 'delta'));
        $this->assertSame([false, false, false, true], array_column($history['rows'], 'current'));

        // Cumulative gap = completed months only: -40 - 10 + 20; Jul's -60 excluded.
        $this->assertSame(-30, $history['gap']);
    }

    public function test_target_resolves_per_month_via_effective_dated_changes(): void
    {
        ClientTargetChange::create(['client_id' => $this->client->id, 'effective_from' => '2026-06-01', 'hours' => 200]);

        $targets = array_column($this->history()['rows'], 'target');

        $this->assertSame([100, 100, 200, 200], $targets);
    }

    public function test_no_target_client_shows_delivered_only_and_no_gap(): void
    {
        $this->client->update(['monthly_target_hours' => null]);
        $this->worklog(3600, '2026-06-15 09:00:00');

        $history = $this->history();

        $this->assertSame(60, $history['rows'][2]['delivered']);
        $this->assertSame([null, null, null, null], array_column($history['rows'], 'target'));
        $this->assertSame([null, null, null, null], array_column($history['rows'], 'delta'));
        $this->assertNull($history['gap']);
    }

    public function test_window_length_honors_the_config_key(): void
    {
        config(['fylla.delivery_history_months' => 1]);

        $months = array_column($this->history()['rows'], 'month');

        $this->assertSame(['Jun 2026', 'Jul 2026'], $months);
    }
}

<?php

namespace Tests\Feature;

use App\Models\Client;
use App\Models\ClientTargetChange;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class ClientTargetChangeTest extends TestCase
{
    use RefreshDatabase;

    public function test_falls_back_to_default_without_changes(): void
    {
        $client = Client::create(['name' => 'Acme', 'monthly_target_hours' => 160]);

        $this->assertSame(160, $client->targetForMonth(CarbonImmutable::parse('2026-07-15')));
    }

    public function test_no_default_and_no_changes_is_null(): void
    {
        $client = Client::create(['name' => 'Acme', 'monthly_target_hours' => null]);

        $this->assertNull($client->targetForMonth(CarbonImmutable::parse('2026-07-15')));
    }

    public function test_latest_change_on_or_before_the_month_wins(): void
    {
        $client = Client::create(['name' => 'Acme', 'monthly_target_hours' => 160]);
        ClientTargetChange::create(['client_id' => $client->id, 'effective_from' => '2026-03-01', 'hours' => 120]);
        ClientTargetChange::create(['client_id' => $client->id, 'effective_from' => '2026-06-01', 'hours' => 200]);

        // Before any change → default.
        $this->assertSame(160, $client->targetForMonth(CarbonImmutable::parse('2026-02-10')));
        // Between changes → the March one persists forward.
        $this->assertSame(120, $client->targetForMonth(CarbonImmutable::parse('2026-05-20')));
        // After the last change → it persists indefinitely.
        $this->assertSame(200, $client->targetForMonth(CarbonImmutable::parse('2026-12-31')));
    }

    public function test_boundary_change_effective_this_month_applies_next_month_does_not(): void
    {
        $client = Client::create(['name' => 'Acme', 'monthly_target_hours' => 160]);
        ClientTargetChange::create(['client_id' => $client->id, 'effective_from' => '2026-07-01', 'hours' => 100]);
        ClientTargetChange::create(['client_id' => $client->id, 'effective_from' => '2026-08-01', 'hours' => 300]);

        // Any day within July resolves to the July change, not August's.
        $this->assertSame(100, $client->targetForMonth(CarbonImmutable::parse('2026-07-01')));
        $this->assertSame(100, $client->targetForMonth(CarbonImmutable::parse('2026-07-31')));
        $this->assertSame(300, $client->targetForMonth(CarbonImmutable::parse('2026-08-01')));
    }

    public function test_change_belongs_to_client(): void
    {
        $client = Client::create(['name' => 'Acme', 'monthly_target_hours' => 160]);
        $change = ClientTargetChange::create(['client_id' => $client->id, 'effective_from' => '2026-07-01', 'hours' => 100]);

        $this->assertTrue($change->client->is($client));
    }
}

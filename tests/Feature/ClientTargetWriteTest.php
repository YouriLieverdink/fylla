<?php

namespace Tests\Feature;

use App\Models\Client;
use App\Models\ClientTargetChange;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

/** Target writes from the client page's history widget (#68). */
class ClientTargetWriteTest extends TestCase
{
    use RefreshDatabase;

    private function client(): Client
    {
        return Client::create(['name' => 'Acme', 'monthly_target_hours' => 160]);
    }

    public function test_store_normalizes_effective_from_to_first_of_month(): void
    {
        $client = $this->client();

        $this->post("/clients/{$client->id}/target-changes", [
            'effective_from' => '2026-07-15',
            'hours' => 120,
        ])->assertRedirect();

        $this->assertDatabaseHas('client_target_changes', [
            'client_id' => $client->id,
            'effective_from' => '2026-07-01',
            'hours' => 120,
        ]);
    }

    public function test_store_for_existing_month_corrects_that_row(): void
    {
        $client = $this->client();
        ClientTargetChange::create(['client_id' => $client->id, 'effective_from' => '2026-07-01', 'hours' => 120]);

        $this->post("/clients/{$client->id}/target-changes", [
            'effective_from' => '2026-07-20',
            'hours' => 140,
        ])->assertRedirect();

        $this->assertSame(1, ClientTargetChange::count());
        $this->assertSame(140, ClientTargetChange::first()->hours);
    }

    public function test_store_rejects_negative_hours(): void
    {
        $client = $this->client();

        $this->post("/clients/{$client->id}/target-changes", [
            'effective_from' => '2026-07-01',
            'hours' => -5,
        ])->assertSessionHasErrors('hours');
    }

    public function test_update_edits_hours_and_normalizes_month(): void
    {
        $client = $this->client();
        $change = ClientTargetChange::create(['client_id' => $client->id, 'effective_from' => '2026-07-01', 'hours' => 120]);

        $this->patch("/target-changes/{$change->id}", [
            'effective_from' => '2026-08-09',
            'hours' => 200,
        ])->assertRedirect();

        $this->assertDatabaseHas('client_target_changes', [
            'id' => $change->id,
            'effective_from' => '2026-08-01',
            'hours' => 200,
        ]);
    }

    public function test_update_moving_onto_an_existing_month_replaces_that_row(): void
    {
        $client = $this->client();
        $july = ClientTargetChange::create(['client_id' => $client->id, 'effective_from' => '2026-07-01', 'hours' => 120]);
        $august = ClientTargetChange::create(['client_id' => $client->id, 'effective_from' => '2026-08-01', 'hours' => 200]);

        $this->patch("/target-changes/{$july->id}", [
            'effective_from' => '2026-08-01',
            'hours' => 150,
        ])->assertRedirect();

        $this->assertSame(1, ClientTargetChange::count());
        $this->assertDatabaseMissing('client_target_changes', ['id' => $august->id]);
        $this->assertDatabaseHas('client_target_changes', ['id' => $july->id, 'effective_from' => '2026-08-01', 'hours' => 150]);
    }

    public function test_destroy_deletes_the_override_and_months_revert(): void
    {
        $client = $this->client();
        $change = ClientTargetChange::create(['client_id' => $client->id, 'effective_from' => '2026-07-01', 'hours' => 120]);

        $this->delete("/target-changes/{$change->id}")->assertRedirect();

        $this->assertDatabaseMissing('client_target_changes', ['id' => $change->id]);
        // Affected months resolve back to the default (or a prior entry).
        $this->assertSame(160, $client->targetForMonth(CarbonImmutable::parse('2026-07-15')));
    }

    public function test_client_page_ships_target_prop_with_changes(): void
    {
        $client = $this->client();
        ClientTargetChange::create(['client_id' => $client->id, 'effective_from' => '2026-08-01', 'hours' => 200]);

        $this->get("/delivery/{$client->id}")
            ->assertInertia(fn ($page) => $page
                ->component('ClientContext')
                ->where('target.clientId', $client->id)
                ->where('target.default', 160)
                ->where('target.changes.0.month', '2026-08')
                ->where('target.changes.0.hours', 200));
    }
}

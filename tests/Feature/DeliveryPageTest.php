<?php

namespace Tests\Feature;

use App\Models\Client;
use App\Models\Project;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Inertia\Testing\AssertableInertia;
use Tests\TestCase;

class DeliveryPageTest extends TestCase
{
    use RefreshDatabase;

    public function test_index_passes_both_cards_and_raw_projects(): void
    {
        $client = Client::create(['name' => 'Acme', 'monthly_target_hours' => 160]);
        Project::create(['kendo_id' => 1, 'name' => 'App', 'billable' => true, 'client_id' => $client->id]);

        $this->get('/delivery')
            ->assertOk()
            ->assertInertia(fn (AssertableInertia $page) => $page
                ->component('Delivery')
                ->has('clients', 1)
                ->where('clients.0.name', 'Acme')
                ->where('clients.0.target', 160)
                ->has('projects', 1)
                ->where('projects.0.client_id', $client->id)
                ->where('projects.0.billable', true));
    }

    public function test_no_target_client_card_has_null_target(): void
    {
        Client::create(['name' => 'Acme', 'monthly_target_hours' => null]);

        $this->get('/delivery')
            ->assertOk()
            ->assertInertia(fn (AssertableInertia $page) => $page
                ->where('clients.0.target', null));
    }

    public function test_footer_target_edit_persists_via_client_route(): void
    {
        $client = Client::create(['name' => 'Acme', 'monthly_target_hours' => null]);

        $this->patch('/clients/'.$client->id, ['monthly_target_hours' => 120])->assertRedirect();

        $this->assertSame(120, $client->fresh()->monthly_target_hours);
    }

    public function test_footer_billable_pill_toggles_via_project_route(): void
    {
        $project = Project::create(['kendo_id' => 1, 'name' => 'App', 'billable' => false]);

        $this->patch('/projects/'.$project->id, ['billable' => true])->assertRedirect();

        $this->assertTrue($project->fresh()->billable);
    }
}

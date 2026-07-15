<?php

namespace Tests\Feature;

use App\Models\Client;
use App\Models\Project;
use App\Models\SyncedWorklog;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Inertia\Testing\AssertableInertia;
use Tests\TestCase;

class ClientManagementTest extends TestCase
{
    use RefreshDatabase;

    private function project(array $overrides = []): Project
    {
        return Project::create(array_merge([
            'kendo_id' => 1, 'name' => 'Acme site', 'billable' => false,
        ], $overrides));
    }

    public function test_index_renders_clients_page_with_projects_and_clients(): void
    {
        $client = Client::create(['name' => 'Acme']);
        $this->project(['client_id' => $client->id]);

        $this->get('/clients')
            ->assertOk()
            ->assertInertia(fn (AssertableInertia $page) => $page
                ->component('Clients')
                ->has('projects', 1)
                ->has('clients', 1)
                ->where('projects.0.client_id', $client->id));
    }

    public function test_assign_and_unassign_project(): void
    {
        $client = Client::create(['name' => 'Acme']);
        $project = $this->project();

        $this->patch('/projects/'.$project->id, ['client_id' => $client->id])->assertRedirect();
        $this->assertSame($client->id, $project->fresh()->client_id);

        $this->patch('/projects/'.$project->id, ['client_id' => null])->assertRedirect();
        $this->assertNull($project->fresh()->client_id);
    }

    public function test_billable_toggle_still_works(): void
    {
        $project = $this->project();

        $this->patch('/projects/'.$project->id, ['billable' => true])->assertRedirect();

        $this->assertTrue($project->fresh()->billable);
    }

    public function test_create_client(): void
    {
        $this->post('/clients', ['name' => 'Acme', 'monthly_target_hours' => 40])->assertRedirect();

        $this->assertDatabaseHas('clients', ['name' => 'Acme', 'monthly_target_hours' => 40]);
    }

    public function test_rename_and_set_clear_target(): void
    {
        $client = Client::create(['name' => 'Acme', 'monthly_target_hours' => 40]);

        $this->patch('/clients/'.$client->id, ['name' => 'Acme Corp'])->assertRedirect();
        $this->patch('/clients/'.$client->id, ['monthly_target_hours' => null])->assertRedirect();

        $fresh = $client->fresh();
        $this->assertSame('Acme Corp', $fresh->name);
        $this->assertNull($fresh->monthly_target_hours);
    }

    public function test_delete_client_nulls_projects_no_cascade(): void
    {
        $client = Client::create(['name' => 'Acme']);
        $project = $this->project(['client_id' => $client->id]);
        SyncedWorklog::create([
            'kendo_worklog_id' => 1, 'kendo_user_id' => 99, 'kendo_project_id' => 1, 'minutes' => 60,
            'started_at' => now()->toDateString().' 09:00:00', 'issue_key' => 'A-1', 'issue_title' => 'T',
        ]);

        $this->delete('/clients/'.$client->id)->assertRedirect();

        $this->assertDatabaseMissing('clients', ['id' => $client->id]);
        $this->assertNull($project->fresh()->client_id);
        $this->assertDatabaseHas('projects', ['id' => $project->id]);
        $this->assertDatabaseCount('synced_worklogs', 1);
    }
}

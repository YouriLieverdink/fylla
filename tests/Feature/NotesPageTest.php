<?php

namespace Tests\Feature;

use App\Models\Client;
use App\Models\Developer;
use App\Models\Project;
use App\Models\SyncedWorklog;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Inertia\Testing\AssertableInertia;
use Tests\TestCase;

/**
 * Notes search page (#70): free-text over synced worklog notes (note + issue
 * key/title, LIKE, newest-first) with client/project/developer/date filters.
 * A team read — deliberately unscoped (ADR-0011).
 */
class NotesPageTest extends TestCase
{
    use RefreshDatabase;

    private function worklog(array $overrides = []): SyncedWorklog
    {
        static $id = 0;

        return SyncedWorklog::create(array_merge([
            'kendo_worklog_id' => ++$id,
            'kendo_project_id' => 1,
            'kendo_user_id' => 10,
            'minutes' => 60,
            'started_at' => '2026-07-01 09:00:00',
            'note' => 'Worked on things',
            'issue_key' => 'ABC-1',
            'issue_title' => 'Some issue',
        ], $overrides));
    }

    public function test_lists_noted_worklogs_newest_first_and_skips_noteless_rows(): void
    {
        Developer::create(['kendo_id' => 10, 'name' => 'Youri']);
        $this->worklog(['note' => 'older', 'started_at' => '2026-07-01 09:00:00']);
        $this->worklog(['note' => 'newer', 'started_at' => '2026-07-02 09:00:00']);
        $this->worklog(['note' => null]);
        $this->worklog(['note' => '']);

        $this->get('/notes')
            ->assertOk()
            ->assertInertia(fn (AssertableInertia $page) => $page
                ->has('rows', 2)
                ->where('rows.0.note', 'newer')
                ->where('rows.0.developer', 'Youri')
                ->where('rows.1.note', 'older')
                ->where('total', 2));
    }

    public function test_search_matches_note_and_issue_key_and_title(): void
    {
        $this->worklog(['note' => 'deploy pipeline fixed']);
        $this->worklog(['note' => 'unrelated', 'issue_key' => 'DEPLOY-9']);
        $this->worklog(['note' => 'unrelated', 'issue_title' => 'Deploy tooling']);
        $this->worklog(['note' => 'nothing to see']);

        $this->get('/notes?q=deploy')
            ->assertInertia(fn (AssertableInertia $page) => $page->has('rows', 3));
    }

    public function test_filters_by_client_project_developer_and_date_range(): void
    {
        $client = Client::create(['name' => 'Acme']);
        Project::create(['kendo_id' => 1, 'name' => 'Managed', 'client_id' => $client->id]);
        Project::create(['kendo_id' => 2, 'name' => 'Other']);

        $this->worklog(['note' => 'match', 'kendo_project_id' => 1, 'kendo_user_id' => 10, 'started_at' => '2026-07-10 09:00:00']);
        $this->worklog(['note' => 'wrong project', 'kendo_project_id' => 2]);
        $this->worklog(['note' => 'wrong developer', 'kendo_user_id' => 99]);
        $this->worklog(['note' => 'too early', 'started_at' => '2026-06-01 09:00:00']);
        $this->worklog(['note' => 'too late', 'started_at' => '2026-08-01 09:00:00']);

        $this->get('/notes?clients[]='.$client->id.'&projects[]=1&developers[]=10&from=2026-07-01&to=2026-07-31')
            ->assertInertia(fn (AssertableInertia $page) => $page
                ->has('rows', 1)
                ->where('rows.0.note', 'match'));
    }

    public function test_client_and_project_filters_accept_multiple_values(): void
    {
        $acme = Client::create(['name' => 'Acme']);
        $beta = Client::create(['name' => 'Beta']);
        Project::create(['kendo_id' => 1, 'name' => 'P1', 'client_id' => $acme->id]);
        Project::create(['kendo_id' => 2, 'name' => 'P2', 'client_id' => $beta->id]);
        Project::create(['kendo_id' => 3, 'name' => 'P3']);

        $this->worklog(['note' => 'in acme', 'kendo_project_id' => 1]);
        $this->worklog(['note' => 'in beta', 'kendo_project_id' => 2]);
        $this->worklog(['note' => 'in neither', 'kendo_project_id' => 3]);

        $this->get('/notes?clients[]='.$acme->id.'&clients[]='.$beta->id)
            ->assertInertia(fn (AssertableInertia $page) => $page->has('rows', 2));

        $this->get('/notes?projects[]=1&projects[]=3')
            ->assertInertia(fn (AssertableInertia $page) => $page->has('rows', 2));
    }

    public function test_developer_filter_accepts_multiple_values(): void
    {
        $this->worklog(['note' => 'by ten', 'kendo_user_id' => 10]);
        $this->worklog(['note' => 'by eleven', 'kendo_user_id' => 11]);
        $this->worklog(['note' => 'by twelve', 'kendo_user_id' => 12]);

        $this->get('/notes?developers[]=10&developers[]=12')
            ->assertInertia(fn (AssertableInertia $page) => $page->has('rows', 2));
    }

    public function test_filter_options_are_scoped_to_the_note_corpus(): void
    {
        $acme = Client::create(['name' => 'Acme']);
        Client::create(['name' => 'Idle']);
        Project::create(['kendo_id' => 1, 'name' => 'Noted', 'client_id' => $acme->id]);
        Project::create(['kendo_id' => 2, 'name' => 'Noteless']);
        Developer::create(['kendo_id' => 10, 'name' => 'Youri']);
        Developer::create(['kendo_id' => 99, 'name' => 'Ghost']);

        $this->worklog(['kendo_project_id' => 1, 'kendo_user_id' => 10]);
        $this->worklog(['kendo_project_id' => 2, 'kendo_user_id' => 99, 'note' => null]);

        $this->get('/notes')
            ->assertInertia(fn (AssertableInertia $page) => $page
                ->has('clients', 1)->where('clients.0.name', 'Acme')
                ->has('projects', 1)->where('projects.0.name', 'Noted')
                ->has('developers', 1)->where('developers.0.name', 'Youri'));
    }
}

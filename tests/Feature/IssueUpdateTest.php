<?php

namespace Tests\Feature;

use App\Models\Issue;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

/**
 * Editing priority (Kendo write-through, ADR-0014) and the local scheduling
 * fields (ADR-0004) from the Worklist via PATCH /issues/{issue}.
 */
class IssueUpdateTest extends TestCase
{
    use RefreshDatabase;

    private function issue(array $overrides = []): Issue
    {
        return Issue::create(array_merge([
            'kendo_id' => 1905, 'key' => 'A-1', 'title' => 'Row',
            'priority' => 'Medium', 'type' => 'Task', 'project_id' => 3,
            'synced_at' => now(),
        ], $overrides));
    }

    public function test_priority_read_modify_write_puts_the_int_and_updates_the_local_label(): void
    {
        $issue = $this->issue();
        Http::fake([
            '*/api/projects/3/issues/1905' => Http::sequence()
                // GET: Kendo's full object, incl. a field the mirror never stores.
                ->push(['id' => 1905, 'priority' => 2, 'description' => 'keep me'], 200)
                // PUT: 2xx
                ->push([], 200),
        ]);

        $this->patch('/issues/'.$issue->id, ['priority' => 'High'])->assertRedirect();

        $this->assertSame('High', $issue->fresh()->priority);

        // GET then PUT, and the PUT carries the mapped int with description intact.
        Http::assertSentInOrder([
            fn ($req) => $req->method() === 'GET' && str_ends_with($req->url(), '/api/projects/3/issues/1905'),
            fn ($req) => $req->method() === 'PUT'
                && $req['priority'] === 1 // High = 1
                && $req['description'] === 'keep me',
        ]);
    }

    public function test_estimate_is_written_back_to_kendo_and_mirrored_locally(): void
    {
        $issue = $this->issue(['estimated_minutes' => 60]);
        Http::fake([
            '*/api/projects/3/issues/1905' => Http::sequence()
                ->push(['id' => 1905, 'priority' => 2, 'estimated_minutes' => 60, 'description' => 'keep me'], 200)
                ->push([], 200),
        ]);

        // 3h = 180 min; priority unchanged so only estimate rides the PUT.
        $this->patch('/issues/'.$issue->id, ['priority' => 'Medium', 'estimated_minutes' => 180])
            ->assertRedirect();

        $this->assertSame(180, $issue->fresh()->estimated_minutes);

        Http::assertSentInOrder([
            fn ($req) => $req->method() === 'GET',
            fn ($req) => $req->method() === 'PUT'
                && $req['estimated_minutes'] === 180
                && $req['description'] === 'keep me',
        ]);
    }

    public function test_kendo_failure_leaves_local_priority_unchanged_and_surfaces_an_error(): void
    {
        $issue = $this->issue(['priority' => 'Medium']);
        Http::fake([
            '*/api/projects/3/issues/1905' => Http::sequence()
                ->push(['id' => 1905, 'priority' => 2], 200)
                ->push([], 500), // PUT fails
        ]);

        $this->patch('/issues/'.$issue->id, ['priority' => 'High'])
            ->assertSessionHasErrors('priority');

        $this->assertSame('Medium', $issue->fresh()->priority);
    }

    public function test_a_failed_priority_write_still_persists_the_scheduling_fields(): void
    {
        $issue = $this->issue(['priority' => 'Medium']);
        Http::fake([
            '*/api/projects/3/issues/1905' => Http::response([], 500), // GET fails
        ]);

        $this->patch('/issues/'.$issue->id, [
            'priority' => 'High',
            'due_date' => '2026-08-01',
            'not_before' => '2026-07-20',
            'up_next' => true,
        ])->assertSessionHasErrors('priority');

        $fresh = $issue->fresh();
        $this->assertSame('Medium', $fresh->priority); // priority isolated
        $this->assertSame('2026-08-01', $fresh->due_date->toDateString());
        $this->assertSame('2026-07-20', $fresh->not_before->toDateString());
        $this->assertTrue($fresh->up_next);
    }

    public function test_local_fields_write_without_touching_kendo_and_non_editable_fields_are_ignored(): void
    {
        Http::fake(); // any provider call would fail the test
        $issue = $this->issue(['title' => 'Original']);

        $this->patch('/issues/'.$issue->id, [
            'up_next' => true,
            'due_date' => '2026-08-01',
            'title' => 'Hacked',   // not whitelisted
            'key' => 'ZZ-9',       // not whitelisted
        ])->assertRedirect();

        $fresh = $issue->fresh();
        $this->assertTrue($fresh->up_next);
        $this->assertSame('2026-08-01', $fresh->due_date->toDateString());
        $this->assertSame('Original', $fresh->title);
        $this->assertSame('A-1', $fresh->key);
        Http::assertNothingSent();
    }
}

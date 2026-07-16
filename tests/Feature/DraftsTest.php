<?php

namespace Tests\Feature;

use App\Jobs\SyncKendoIssues;
use App\Models\Draft;
use App\Models\Issue;
use App\Models\Project;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Facades\Http;
use Inertia\Testing\AssertableInertia;
use Tests\TestCase;

/**
 * Fylla-native drafts (ADR-0012): a third work source, ranked by the same
 * scorer and untouched by the Kendo sync.
 */
class DraftsTest extends TestCase
{
    use RefreshDatabase;

    public function test_capture_creates_a_medium_priority_draft(): void
    {
        $this->post('/drafts', ['title' => 'Email the client'])->assertRedirect();

        $draft = Draft::sole();
        $this->assertSame('Email the client', $draft->title);
        $this->assertSame('Medium', $draft->priority);
    }

    public function test_draft_ranks_inline_alongside_provider_items(): void
    {
        // A low-priority open issue and a pinned draft — the draft should rank first.
        Issue::create([
            'kendo_id' => 1, 'key' => 'A-1', 'title' => 'Low issue',
            'priority' => 'Lowest', 'synced_at' => now(),
        ]);
        Cache::forever('kendo.synced_at', now()->toJSON());
        Draft::create(['title' => 'Pinned draft', 'up_next' => true]);

        $this->get('/')
            ->assertOk()
            ->assertInertia(fn (AssertableInertia $page) => $page
                ->component('Worklist')
                ->has('items', 2)
                ->where('items.0.kind', 'draft')
                ->where('items.0.title', 'Pinned draft')
                ->where('items.1.kind', 'issue'));
    }

    public function test_kendo_sync_leaves_drafts_intact(): void
    {
        $draft = Draft::create(['title' => 'Talk to Sam']);

        // A my-issues sync that returns an empty, non-truncated feed deletes
        // absent issues — but must never touch the separate drafts table.
        Http::fake([
            '*/api/projects/*/issues' => Http::response([]),
            '*/api/issues/my' => Http::response([
                'data' => [],
                'meta' => ['truncated' => false, 'count' => 0, 'limit' => 500],
            ]),
        ]);

        SyncKendoIssues::dispatchSync();

        $this->assertDatabaseHas('drafts', ['id' => $draft->id, 'title' => 'Talk to Sam']);
    }

    public function test_draft_edits_and_deletes_are_local(): void
    {
        $draft = Draft::create(['title' => 'Review contract']);

        $this->patch("/drafts/{$draft->id}", ['priority' => 'Highest', 'up_next' => true])
            ->assertRedirect();
        $this->assertSame('Highest', $draft->fresh()->priority);
        $this->assertTrue($draft->fresh()->up_next);

        $this->delete("/drafts/{$draft->id}")->assertRedirect();
        $this->assertDatabaseMissing('drafts', ['id' => $draft->id]);
    }

    public function test_promote_creates_a_kendo_issue_and_removes_the_draft(): void
    {
        Project::create(['kendo_id' => 3, 'name' => 'Acme']);
        $draft = Draft::create(['title' => 'Formalize this', 'priority' => 'High', 'up_next' => true]);

        // create resolves the first lane + active sprint, then POSTs; the follow-up
        // sync's my-issues feed returns it as an ordinary assigned, timeable issue.
        Http::fake(function ($request) {
            $url = $request->url();
            if (str_contains($url, '/lanes')) {
                return Http::response([['id' => 11, 'order' => 2], ['id' => 10, 'order' => 1]]);
            }
            if (str_contains($url, '/sprints')) {
                return Http::response([['id' => 77, 'status' => 1]]);
            }
            if (str_contains($url, '/api/issues/my')) {
                return Http::response([
                    'data' => [[
                        'id' => 5001, 'key' => 'ACME-1', 'title' => 'Formalize this',
                        'priority' => 1, 'type' => 2, 'lane_id' => 10, 'project_id' => 3,
                        'epic_id' => null, 'updated_at' => '2026-07-16T08:00:00+00:00',
                    ]],
                    'meta' => ['truncated' => false, 'count' => 1, 'limit' => 500],
                ]);
            }
            if ($request->method() === 'POST') {
                return Http::response(['id' => 5001, 'key' => 'ACME-1']);
            }

            return Http::response([]); // per-project estimates feed
        });

        $this->post("/drafts/{$draft->id}/promote", ['project_id' => 3])->assertRedirect();

        Http::assertSent(fn ($r) => $r->method() === 'POST'
            && str_contains($r->url(), '/api/projects/3/issues')
            && $r['title'] === 'Formalize this'
            && $r['lane_id'] === 10  // lowest order wins
            && $r['sprint_id'] === 77 // the active sprint
            && $r['type'] === 2);

        $this->assertDatabaseMissing('drafts', ['id' => $draft->id]);
        // Mirrored in via the inline sync — a normal, timeable Kendo issue.
        $issue = Issue::where('kendo_id', 5001)->sole();
        $this->assertSame('ACME-1', $issue->key);
        $this->assertSame(3, $issue->project_id);
    }

    public function test_promote_failure_leaves_the_draft_intact(): void
    {
        Project::create(['kendo_id' => 3, 'name' => 'Acme']);
        $draft = Draft::create(['title' => 'Keep me']);

        Http::fake(['*' => Http::response('boom', 500)]);

        $this->post("/drafts/{$draft->id}/promote", ['project_id' => 3])
            ->assertSessionHasErrors('promote');

        $this->assertDatabaseHas('drafts', ['id' => $draft->id]);
        $this->assertDatabaseMissing('issues', ['kendo_id' => 5001]);
    }
}

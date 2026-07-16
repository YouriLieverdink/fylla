<?php

namespace Tests\Feature;

use App\Jobs\SyncKendoIssues;
use App\Models\Draft;
use App\Models\Issue;
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
}

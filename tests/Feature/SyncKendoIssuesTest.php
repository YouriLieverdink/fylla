<?php

namespace Tests\Feature;

use App\Jobs\SyncKendoIssues;
use App\Models\Issue;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class SyncKendoIssuesTest extends TestCase
{
    use RefreshDatabase;

    /** Body of one my-issues response (real REST shape: data + meta). */
    private function feed(array $issues, bool $truncated = false): array
    {
        return [
            'data' => $issues,
            'meta' => ['truncated' => $truncated, 'count' => count($issues), 'limit' => 500],
        ];
    }

    /** Stub the endpoint to return each feed in turn on successive calls. */
    private function fakeFeeds(array ...$feeds): void
    {
        $sequence = Http::sequence();
        foreach ($feeds as $feed) {
            $sequence->push($feed, 200);
        }
        Http::fake(['*/api/issues/my' => $sequence]);
    }

    private function payload(int $id, string $key, array $overrides = []): array
    {
        return array_merge([
            'id' => $id,
            'key' => $key,
            'title' => "Title $id",
            'priority' => 2, // Medium
            'type' => 2,     // Task
            'lane_id' => 9,
            'project_id' => 3,
            'epic_id' => null,
            'updated_at' => '2026-07-01T08:25:23+00:00',
        ], $overrides);
    }

    public function test_upserts_mirror_fields_and_stamps_synced_at(): void
    {
        $this->fakeFeeds($this->feed([$this->payload(1905, 'SOHY-0173')]));

        SyncKendoIssues::dispatchSync();

        $issue = Issue::sole();
        $this->assertSame(1905, $issue->kendo_id);
        $this->assertSame('SOHY-0173', $issue->key);
        $this->assertSame('Medium', $issue->priority);
        $this->assertNotNull($issue->synced_at);
    }

    public function test_updates_existing_issue_instead_of_duplicating(): void
    {
        $this->fakeFeeds(
            $this->feed([$this->payload(1905, 'SOHY-0173')]),
            $this->feed([$this->payload(1905, 'SOHY-0173', ['title' => 'Renamed'])]),
        );

        SyncKendoIssues::dispatchSync();
        SyncKendoIssues::dispatchSync();

        $this->assertSame(1, Issue::count());
        $this->assertSame('Renamed', Issue::sole()->title);
    }

    public function test_deletes_issues_absent_from_a_complete_feed(): void
    {
        $this->fakeFeeds(
            $this->feed([$this->payload(1, 'A-1'), $this->payload(2, 'A-2')]),
            $this->feed([$this->payload(1, 'A-1')]), // issue 2 gone
        );

        SyncKendoIssues::dispatchSync();
        SyncKendoIssues::dispatchSync();

        $this->assertSame([1], Issue::pluck('kendo_id')->all());
    }

    public function test_does_not_delete_absent_issues_when_feed_is_truncated(): void
    {
        $this->fakeFeeds(
            $this->feed([$this->payload(1, 'A-1'), $this->payload(2, 'A-2')]),
            $this->feed([$this->payload(1, 'A-1')], truncated: true),
        );

        SyncKendoIssues::dispatchSync();
        SyncKendoIssues::dispatchSync();

        $this->assertSame(2, Issue::count());
    }

    public function test_preserves_fylla_owned_columns_across_sync(): void
    {
        $this->fakeFeeds(
            $this->feed([$this->payload(1, 'A-1')]),
            $this->feed([$this->payload(1, 'A-1', ['title' => 'Changed'])]),
        );

        SyncKendoIssues::dispatchSync();
        Issue::where('kendo_id', 1)->update(['due_date' => '2026-08-01', 'up_next' => true]);
        SyncKendoIssues::dispatchSync();

        $issue = Issue::sole();
        $this->assertSame('Changed', $issue->title);
        $this->assertSame('2026-08-01', $issue->due_date->toDateString());
        $this->assertTrue($issue->up_next);
    }
}

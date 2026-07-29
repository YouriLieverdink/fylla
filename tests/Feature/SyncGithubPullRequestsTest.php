<?php

namespace Tests\Feature;

use App\GitHub\Client as GitHubClient;
use App\Jobs\SyncGithubPullRequests;
use App\Models\PullRequest;
use App\Models\Timer;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Http;
use Inertia\Testing\AssertableInertia;
use Tests\TestCase;

class SyncGithubPullRequestsTest extends TestCase
{
    use RefreshDatabase;

    public function test_sync_unions_direct_team_and_authored_action_queries_without_duplicates(): void
    {
        config(['fylla.github_pr_queries' => [
            'org:Back-to-code review-requested:@me',
            'org:Back-to-code team-review-requested:Back-to-code/reviewers',
            'org:Back-to-code author:@me review:changes_requested',
        ]]);

        $existing = PullRequest::create([
            'github_id' => 101,
            'number' => 12,
            'repo' => 'Back-to-code/app',
            'title' => 'Old title',
            'url' => 'https://github.com/Back-to-code/app/pull/12',
            'state' => 'open',
            'kendo_issue_id' => 44,
            'kendo_project_id' => 7,
            'kendo_key' => 'ABC-12',
            'resolved_at' => now()->subDay(),
        ]);

        Http::fakeSequence()
            ->push($this->searchResponse([$this->githubPr(101, 12, 'ABC-12 Direct review')]))
            ->push($this->searchResponse([
                $this->githubPr(101, 12, 'ABC-12 Direct review'),
                $this->githubPr(102, 13, 'ABC-13 Team review'),
            ]))
            ->push($this->searchResponse([$this->githubPr(103, 14, 'ABC-14 Changes requested')]));

        app(SyncGithubPullRequests::class)->handle(app(GitHubClient::class));

        $this->assertDatabaseCount('pull_requests', 3);
        $existing->refresh();
        $this->assertTrue($existing->actionable);
        $this->assertSame('ABC-12 Direct review', $existing->title);
        $this->assertSame('ABC-12', $existing->kendo_key);
        $this->assertSame(44, $existing->kendo_issue_id);
        Http::assertSentCount(3);
    }

    public function test_a_pr_no_longer_returned_by_the_actionable_queries_leaves_the_worklist(): void
    {
        config(['fylla.github_pr_queries' => ['org:Back-to-code review-requested:@me']]);

        $pr = PullRequest::create([
            'github_id' => 101,
            'number' => 12,
            'repo' => 'Back-to-code/app',
            'title' => 'ABC-12 Review me',
            'url' => 'https://github.com/Back-to-code/app/pull/12',
            'state' => 'open',
            'synced_at' => now()->subHour(),
        ]);
        Timer::create([
            'timeable_type' => PullRequest::class,
            'timeable_id' => $pr->id,
            'stopped_at' => now(),
        ]);

        Http::fake([
            'api.github.com/search/issues*' => Http::response([
                'items' => [],
                'total_count' => 0,
                'incomplete_results' => false,
            ]),
        ]);

        app(SyncGithubPullRequests::class)->handle(app(GitHubClient::class));

        // Timer history retains the mirror row, but a submitted review removes
        // the explicit review request, so it is no longer actionable.
        $this->assertDatabaseHas('pull_requests', ['id' => $pr->id]);
        $this->get('/')
            ->assertOk()
            ->assertInertia(fn (AssertableInertia $page) => $page->has('items', 0));
    }

    public function test_a_truncated_search_does_not_deactivate_an_absent_pr(): void
    {
        config(['fylla.github_pr_queries' => ['org:Back-to-code review-requested:@me']]);

        $pr = PullRequest::create([
            'github_id' => 101,
            'number' => 12,
            'repo' => 'Back-to-code/app',
            'title' => 'ABC-12 Review me',
            'url' => 'https://github.com/Back-to-code/app/pull/12',
            'state' => 'open',
            'synced_at' => now()->subHour(),
        ]);

        Http::fake([
            'api.github.com/search/issues*' => Http::response([
                'items' => [],
                'total_count' => 1,
                'incomplete_results' => true,
            ]),
        ]);

        app(SyncGithubPullRequests::class)->handle(app(GitHubClient::class));

        $this->assertTrue($pr->refresh()->actionable);
        $this->get('/')->assertInertia(fn (AssertableInertia $page) => $page
            ->has('items', 1)
            ->where('items.0.number', 12));
    }

    /** @param array<int, array<string, mixed>> $items */
    private function searchResponse(array $items): array
    {
        return [
            'items' => $items,
            'total_count' => count($items),
            'incomplete_results' => false,
        ];
    }

    /** @return array<string, mixed> */
    private function githubPr(int $id, int $number, string $title): array
    {
        return [
            'id' => $id,
            'number' => $number,
            'repository_url' => 'https://api.github.com/repos/Back-to-code/app',
            'title' => $title,
            'body' => null,
            'html_url' => "https://github.com/Back-to-code/app/pull/{$number}",
            'state' => 'open',
            'created_at' => '2026-07-29T10:00:00Z',
        ];
    }
}

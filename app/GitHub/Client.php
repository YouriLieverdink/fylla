<?php

namespace App\GitHub;

use Illuminate\Http\Client\Factory as HttpFactory;
use Illuminate\Http\Client\PendingRequest;

/**
 * Thin REST wrapper around the GitHub API (github.com). Bearer-authenticated
 * with a PAT from config/services.php (github.token); `@me` in queries resolves
 * to the token's owner.
 */
class Client
{
    public function __construct(
        private HttpFactory $http,
        private string $token,
    ) {}

    private function request(): PendingRequest
    {
        return $this->http
            ->baseUrl('https://api.github.com')
            ->withToken($this->token)
            ->withHeaders([
                'Accept' => 'application/vnd.github+json',
                'X-GitHub-Api-Version' => '2022-11-28',
            ]);
    }

    /**
     * Open PRs matching a GitHub search query (`is:pr is:open` is prepended, so
     * the query carries only the filter, e.g. `review-requested:@me`). Title and
     * body ride along in the search feed and are scanned for the linked Kendo
     * key (ADR-0009); the branch name would need a repo-scoped call the org
     * blocks for classic PATs, so it is not read. `truncated` mirrors the issues
     * sync: absence from a capped page is not conclusive, so the reconcile-delete
     * is skipped when set.
     *
     * @return array{prs: array<int, array{github_id:int, number:int, repo:string, title:string, body:?string, url:string, state:string, opened_at:?string}>, truncated: bool}
     */
    public function searchPullRequests(string $query): array
    {
        $body = $this->request()
            ->get('/search/issues', ['q' => trim("is:pr is:open {$query}"), 'per_page' => 100])
            ->throw()
            ->json();

        $items = $body['items'] ?? [];
        $prs = array_map(fn (array $row) => [
            'github_id' => $row['id'],
            'number' => $row['number'],
            // repository_url = https://api.github.com/repos/{owner}/{repo}
            'repo' => str_replace('https://api.github.com/repos/', '', $row['repository_url']),
            'title' => $row['title'],
            'body' => $row['body'] ?? null,
            'url' => $row['html_url'],
            'state' => $row['state'],
            // Feeds the worklist synthetic due (ADR-0013); nullable if absent.
            'opened_at' => $row['created_at'] ?? null,
        ], $items);

        // ponytail: single page of 100. Add pagination if a query ever exceeds it.
        return [
            'prs' => $prs,
            'truncated' => ($body['incomplete_results'] ?? false)
                || ($body['total_count'] ?? 0) > count($items),
        ];
    }
}

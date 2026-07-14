<?php

namespace App\Jobs;

use App\GitHub\Client as GitHubClient;
use App\Models\PullRequest;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;
use Illuminate\Support\Facades\Cache;

/**
 * Pull open GitHub PRs relevant to the user into the local `pull_requests`
 * table (ADR-0009).
 *
 * Unions the search feed over each query in config('fylla.github_pr_queries'),
 * recomputes the suggested Kendo key from each PR's title/body, and upserts
 * GitHub-mirror fields on `github_id` — never touching the Fylla-owned
 * resolution columns. PRs absent from the feed are reconcile-deleted UNLESS they
 * carry local timer history, and never when the feed came back truncated (the
 * exact rule SyncKendoIssues uses).
 */
class SyncGithubPullRequests implements ShouldQueue
{
    use Queueable;

    private const KEY_PATTERN = '/[A-Z][A-Z0-9]*-\d+/';

    public function handle(GitHubClient $github): void
    {
        $now = now();

        // Union across queries, de-duplicating by GitHub PR id.
        $prs = [];
        $truncated = false;
        foreach (config('fylla.github_pr_queries', []) as $query) {
            $result = $github->searchPullRequests($query);
            $truncated = $truncated || $result['truncated'];
            foreach ($result['prs'] as $pr) {
                $prs[$pr['github_id']] = $pr;
            }
        }

        $seen = [];
        foreach ($prs as $pr) {
            $seen[] = $pr['github_id'];

            PullRequest::updateOrCreate(
                ['github_id' => $pr['github_id']],
                [
                    'number' => $pr['number'],
                    'repo' => $pr['repo'],
                    'title' => $pr['title'],
                    'url' => $pr['url'],
                    'state' => $pr['state'],
                    'suggested_key' => $this->parseKey($pr['title'], $pr['body']),
                    'synced_at' => $now,
                ],
            );
        }

        if (! $truncated) {
            PullRequest::whereNotIn('github_id', $seen)
                ->whereDoesntHave('timers')
                ->delete();
        }

        Cache::forever('github.synced_at', $now->toJSON());
    }

    /** Parse the linked Kendo issue key: PR title first, then body (ADR-0009). */
    private function parseKey(?string $title, ?string $body): ?string
    {
        foreach ([$title, $body] as $source) {
            if ($source !== null && preg_match(self::KEY_PATTERN, $source, $m)) {
                return $m[0];
            }
        }

        return null;
    }
}

<?php

namespace App\Http\Controllers;

use App\Kendo\Client as KendoClient;
use App\Models\PullRequest;
use App\Services\TimerService;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;

class PullRequestController extends Controller
{
    public function __construct(private readonly TimerService $timers) {}

    /**
     * Live Kendo issue search for the manual-pick fallback (ADR-0009 exception).
     * Returns candidate {id, key, title, project_id} rows as JSON.
     */
    public function candidates(Request $request, KendoClient $kendo): JsonResponse
    {
        $q = trim((string) $request->query('q'));
        if ($q === '') {
            return response()->json([]);
        }

        return response()->json($kendo->searchIssues($q));
    }

    /**
     * Resolve a PR to its linked Kendo issue by key (confirm of the suggested
     * key, or a manually picked one). The key is re-searched live and must match
     * exactly — the authoritative coordinates come from Kendo, not the client.
     */
    public function resolve(Request $request, PullRequest $pullRequest, KendoClient $kendo): RedirectResponse
    {
        $request->validate(['key' => ['required', 'string']]);
        $key = trim((string) $request->input('key'));

        $issue = collect($kendo->searchIssues($key))->firstWhere('key', $key);
        if (! $issue) {
            return back()->withErrors(['resolve' => "No Kendo issue found for {$key}."]);
        }

        $pullRequest->update([
            'kendo_issue_id' => $issue['id'],
            'kendo_project_id' => $issue['project_id'],
            'kendo_key' => $issue['key'],
            'resolved_at' => now(),
        ]);

        return back();
    }

    /** Start a timer on a PR — gated until it is resolved (ADR-0009). */
    public function startTimer(PullRequest $pullRequest): RedirectResponse
    {
        if ($pullRequest->resolved_at === null) {
            return back()->withErrors(['timer' => 'Resolve the linked Kendo issue first.']);
        }

        $this->timers->start($pullRequest);

        return back();
    }
}

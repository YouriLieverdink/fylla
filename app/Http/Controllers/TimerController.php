<?php

namespace App\Http\Controllers;

use App\Kendo\Client as KendoClient;
use App\Models\Issue;
use App\Services\TimerService;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;

class TimerController extends Controller
{
    public function __construct(private readonly TimerService $timers) {}

    public function start(Request $request): RedirectResponse
    {
        $issue = Issue::findOrFail($request->integer('issue_id'));
        $this->timers->start($issue);

        return back();
    }

    /**
     * Ad-hoc timing (ADR-0015): time any Kendo issue that isn't on the worklist
     * (unassigned PM tasks, reviews of others' tickets). The key is re-searched
     * live for authoritative coordinates (matches the PR resolve path), stored as
     * an Issue row with synced_at left unstamped so it never renders as a card,
     * then timed. Transient: it exists only as the running timer.
     */
    public function adhoc(Request $request, KendoClient $kendo): RedirectResponse
    {
        $request->validate(['key' => ['required', 'string']]);
        $key = trim((string) $request->input('key'));

        $hit = collect($kendo->searchIssues($key))->firstWhere('key', $key);
        if (! $hit) {
            return back()->withErrors(['adhoc' => "No Kendo issue found for {$key}."]);
        }

        DB::transaction(function () use ($hit) {
            // No synced_at: keeps it off the worklist (ADR-0015). updateOrCreate
            // reuses the row (and its timer/worklog history) if picked again.
            $issue = Issue::updateOrCreate(
                ['kendo_id' => $hit['id']],
                ['key' => $hit['key'], 'title' => $hit['title'], 'project_id' => $hit['project_id']],
            );
            $this->timers->start($issue);
        });

        return back();
    }

    public function pause(): RedirectResponse
    {
        $this->timers->pause();

        return back();
    }

    public function resume(): RedirectResponse
    {
        $this->timers->resume();

        return back();
    }

    public function stop(): RedirectResponse
    {
        $this->timers->stop();

        return back();
    }

    public function startTime(Request $request): RedirectResponse
    {
        $request->validate(['time' => ['required', 'date_format:H:i']]);

        try {
            $this->timers->setStartTime((string) $request->string('time'));
        } catch (\RuntimeException $e) {
            return back()->withErrors(['time' => $e->getMessage()]);
        }

        return back();
    }

    public function note(Request $request): RedirectResponse
    {
        $this->timers->addNote((string) $request->input('text', ''));

        return back();
    }
}

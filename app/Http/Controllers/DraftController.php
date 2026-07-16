<?php

namespace App\Http\Controllers;

use App\Jobs\SyncKendoIssues;
use App\Kendo\Client as KendoClient;
use App\Models\Draft;
use App\Services\WorklistScorer;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Validation\Rule;

/**
 * Fylla-native drafts (ADR-0012). Entirely local: no Kendo write-through, no
 * sync. All fields are Fylla-owned, so every edit goes straight to the column.
 */
class DraftController extends Controller
{
    /** One-gesture capture from the Worklist: a title is all it takes. */
    public function store(Request $request): RedirectResponse
    {
        $data = $request->validate(['title' => ['required', 'string', 'max:255']]);

        Draft::create($data); // priority defaults to Medium at the column

        return back();
    }

    public function update(Request $request, Draft $draft): RedirectResponse
    {
        $data = $request->validate([
            'title' => ['sometimes', 'string', 'max:255'],
            'priority' => ['sometimes', Rule::in(array_keys(WorklistScorer::PRIORITY_RANK))],
            'up_next' => ['sometimes', 'boolean'],
            'due_date' => ['sometimes', 'nullable', 'date'],
            'not_before' => ['sometimes', 'nullable', 'date'],
        ]);

        $draft->update($data);

        return back();
    }

    public function destroy(Draft $draft): RedirectResponse
    {
        $draft->delete();

        return back();
    }

    /**
     * Promote a draft into a real Kendo issue (ADR-0012). One-way: the moment
     * Kendo has the issue the promotion is done, so the draft is removed. A
     * create failure leaves the draft intact and surfaces the error. The inline
     * sync just mirrors the new issue in immediately (so it appears and is
     * timeable at once); the scheduled sync reconciles regardless (ADR-0003),
     * so a failure there is non-fatal.
     */
    public function promote(Request $request, Draft $draft, KendoClient $kendo): RedirectResponse
    {
        $data = $request->validate([
            'project_id' => ['required', 'integer', 'exists:projects,kendo_id'],
        ]);

        $assignee = config('fylla.kendo_user_id');

        try {
            // Assigned to the user so the new issue returns in the my-issues feed
            // and mirrors in like any other issue.
            $kendo->createIssue(
                $data['project_id'],
                $draft->title,
                KendoClient::priorityToInt($draft->priority),
                $assignee !== null ? (int) $assignee : null,
            );
        } catch (\Throwable $e) {
            return back()->withErrors(['promote' => 'Could not create the Kendo issue.']);
        }

        try {
            SyncKendoIssues::dispatchSync();
        } catch (\Throwable $e) {
            // Kendo already has the issue; the scheduled sync will mirror it.
        }

        $draft->delete();

        return back();
    }
}

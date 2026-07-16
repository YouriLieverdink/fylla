<?php

namespace App\Http\Controllers;

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
}

<?php

namespace App\Estimation;

use App\Models\FinishedIssue;
use App\Models\Project;
use Illuminate\Support\Collection;

/**
 * Personal estimation feedback loop (issue #17). Compares each finished issue's
 * Kendo estimate against the hours logged against it, and rolls that up into a
 * single bias figure (positive % = you underestimate).
 *
 * Reads the `finished_issues` mirror (Done-lane issues assigned to the user,
 * synced by SyncKendoFinishedIssues) — never local timer worklogs, which are
 * only a partial record. Actual = the issue's Kendo `logged_minutes` (the
 * authoritative total, since Fylla worklogs post to Kendo too). Estimates are
 * hours, so the comparison is direct.
 */
class EstimationReport
{
    /** Rolling bias is computed over at most this many recent finished issues. */
    private const WINDOW = 20;

    /**
     * @param  array<int,int>  $projectIds  Empty = every project.
     * @return array{bias: array<string,mixed>, issues: array<int,array<string,mixed>>, projects: array<int,array{id:int,name:?string}>, projectIds: array<int,int>}
     */
    public function generate(array $projectIds = []): array
    {
        $projectNames = Project::pluck('name', 'kendo_id');

        // Most-recently-worked first; issues never timed sort last.
        $finished = FinishedIssue::query()
            ->when($projectIds, fn ($q) => $q->whereIn('project_id', $projectIds))
            ->orderByRaw('last_worked_at is null, last_worked_at desc')
            ->get();

        return [
            'bias' => $this->bias($finished),
            'issues' => $finished->map(fn (FinishedIssue $i) => $this->row($i, $projectNames))->all(),
            'projects' => $this->projectOptions($projectNames),
            'projectIds' => $projectIds,
        ];
    }

    /** @param  Collection<int,FinishedIssue>  $finished */
    private function bias(Collection $finished): array
    {
        // Only issues that carry an estimate can contribute a bias.
        $sample = $finished->filter(fn (FinishedIssue $i) => (int) $i->estimated_minutes > 0)->take(self::WINDOW);

        $estimate = (int) $sample->sum('estimated_minutes');
        $actual = (int) $sample->sum('logged_minutes');

        return [
            'sampleSize' => $sample->count(),
            'estimateHours' => $this->hours($estimate),
            'actualHours' => $this->hours($actual),
            'pct' => $this->biasPct($estimate, $actual),
        ];
    }

    /** @param  Collection<int|string,string>  $projectNames */
    private function row(FinishedIssue $issue, Collection $projectNames): array
    {
        $estimate = (int) $issue->estimated_minutes;
        $actual = (int) $issue->logged_minutes;

        return [
            'key' => $issue->key,
            'title' => $issue->title,
            'project' => $projectNames[$issue->project_id] ?? null,
            'kendo_url' => $issue->kendo_url,
            'estimateHours' => $estimate > 0 ? $this->hours($estimate) : null,
            'actualHours' => $this->hours($actual),
            'biasPct' => $this->biasPct($estimate, $actual),
            'lastWorked' => $issue->last_worked_at
                ?->setTimezone(config('fylla.display_timezone'))->format('M j, Y'),
        ];
    }

    /**
     * Slice options = every project that has a finished issue (unfiltered, so the
     * dropdown doesn't collapse after you pick one), alphabetical.
     *
     * @param  Collection<int|string,string>  $projectNames
     * @return array<int,array{id:int,name:?string}>
     */
    private function projectOptions(Collection $projectNames): array
    {
        return FinishedIssue::distinct()
            ->pluck('project_id')
            ->filter()
            ->map(fn (int $id) => ['id' => $id, 'name' => $projectNames[$id] ?? null])
            ->sortBy('name')
            ->values()
            ->all();
    }

    /** Minutes → hours, one decimal. */
    private function hours(int $minutes): float
    {
        return round($minutes / 60, 1);
    }

    /** +% = logged more than estimated (underestimated); −% = overestimated. Null without an estimate. */
    private function biasPct(int $estimate, int $actual): ?int
    {
        return $estimate > 0 ? (int) round(($actual - $estimate) / $estimate * 100) : null;
    }
}

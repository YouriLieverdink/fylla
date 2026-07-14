<?php

namespace App\Kendo;

use Illuminate\Http\Client\Factory as HttpFactory;
use Illuminate\Http\Client\PendingRequest;

/**
 * Thin REST wrapper around the Kendo API. Bearer-authenticated; base URL and
 * token come from config/services.php (KENDO_BASE_URL / KENDO_TOKEN).
 */
class Client
{
    private const PRIORITIES = [0 => 'Highest', 1 => 'High', 2 => 'Medium', 3 => 'Low', 4 => 'Lowest'];

    private const TYPES = [0 => 'Feature', 1 => 'Bug', 2 => 'Task'];

    public function __construct(
        private HttpFactory $http,
        private string $baseUrl,
        private string $token,
    ) {
    }

    private function request(): PendingRequest
    {
        return $this->http
            ->baseUrl(rtrim($this->baseUrl, '/'))
            ->withToken($this->token)
            ->acceptJson();
    }

    /**
     * Fetch the current user's open issues (all projects, excludes the done
     * lane, capped). The feed nests rows under `data` and paging state under
     * `meta`; priority/type arrive as integers and are mapped to labels here.
     *
     * @return array{issues: array<int, array<string, mixed>>, truncated: bool}
     */
    public function getMyIssues(): array
    {
        $body = $this->request()->get('/api/issues/my')->throw()->json();

        $issues = array_map(fn (array $row) => [
            'id' => $row['id'],
            'key' => $row['key'],
            'title' => $row['title'],
            'priority' => self::PRIORITIES[$row['priority']] ?? null,
            'type' => self::TYPES[$row['type']] ?? null,
            'lane_id' => $row['lane_id'] ?? null,
            'project_id' => $row['project_id'] ?? null,
            'epic_id' => $row['epic_id'] ?? null,
            'updated_at' => $row['updated_at'] ?? null,
        ], $body['data'] ?? []);

        return [
            'issues' => $issues,
            'truncated' => (bool) ($body['meta']['truncated'] ?? false),
        ];
    }

    /**
     * Estimate mirror fields per issue for one project, keyed by issue id. The
     * lean my-issues feed omits estimated/remaining minutes; the per-project
     * issues feed carries them (remaining is server-computed = estimate − logged).
     *
     * @return array<int, array{estimated_minutes: ?int, remaining_minutes: ?int}>
     */
    public function getProjectEstimates(int $projectId): array
    {
        $body = $this->request()->get("/api/projects/{$projectId}/issues")->throw()->json();
        $rows = $body['data'] ?? $body;

        $out = [];
        foreach ($rows as $row) {
            $out[$row['id']] = [
                'estimated_minutes' => $row['estimated_minutes'] ?? null,
                'remaining_minutes' => $row['remaining_minutes'] ?? null,
            ];
        }

        return $out;
    }

    /**
     * Live global issue search by key/text (ADR-0009 resolution path). The PR's
     * linked issue is usually not the user's, so it is absent from the local
     * mirror — this reads across all Kendo issues, a deliberate ADR-0003
     * exception bounded to key→coordinate resolution.
     *
     * @return array<int, array{id:int, key:string, title:?string, project_id:?int}>
     */
    public function searchIssues(string $query): array
    {
        $body = $this->request()->get('/api/issues/search', ['query' => $query])->throw()->json();
        $rows = $body['data'] ?? $body;

        return array_map(fn (array $row) => [
            'id' => $row['id'],
            'key' => $row['key'],
            'title' => $row['title'] ?? null,
            'project_id' => $row['project_id'] ?? null,
        ], $rows);
    }

    /**
     * All Kendo projects (mirror source for the local `projects` table).
     *
     * @return array<int, array{id: int, name: string, code: ?string}>
     */
    public function getProjects(): array
    {
        $body = $this->request()->get('/api/projects')->throw()->json();
        $rows = $body['data'] ?? $body;

        return array_map(fn (array $row) => [
            'id' => $row['id'],
            'name' => $row['name'],
            'code' => $row['code'] ?? null,
        ], $rows);
    }

    /**
     * Time entries in a date window (whole team — the admin token has no user
     * filter, so callers filter client-side). Dates are inclusive Y-m-d.
     *
     * @return array<int, array{id: int, user_id: ?int, issue_id: ?int, project_id: ?int, minutes: int, started_at: string, note: ?string, issue_key: ?string, issue_title: ?string}>
     */
    public function getTimeEntries(string $from, string $to): array
    {
        $body = $this->request()
            ->get('/api/time-entries', ['start_date' => $from, 'end_date' => $to])
            ->throw()
            ->json();
        $rows = $body['data'] ?? $body;

        return array_map(fn (array $row) => [
            'id' => $row['id'],
            'user_id' => $row['user_id'] ?? null,
            'issue_id' => $row['issue_id'] ?? null,
            'project_id' => $row['project_id'] ?? null,
            'minutes' => $row['minutes_spent'] ?? 0,
            'started_at' => $row['started_at'],
            'note' => $row['note'] ?? null,
            'issue_key' => $row['issue_key'] ?? null,
            'issue_title' => $row['issue_title'] ?? null,
        ], $rows);
    }

    /**
     * Log a time entry against an issue. One entry per local Worklog (ADR-0005);
     * returns the Kendo entry id.
     */
    public function postWorklog(int $projectId, int $issueId, int $minutes, string $startedAt, ?string $note): int
    {
        return (int) $this->request()
            ->post("/api/projects/{$projectId}/issues/{$issueId}/time-entries", [
                'minutes_spent' => $minutes,
                'started_at' => $startedAt,
                'note' => $note,
            ])
            ->throw()
            ->json('id');
    }
}

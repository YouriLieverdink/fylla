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
    ) {}

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
     * All of a project's issues (open AND done), with the mirror fields the team
     * issue sync needs (issue #55): the estimate, the issue's logged (actual)
     * minutes, its lane, assignee, and active-sprint membership. One call returns
     * the whole project; callers classify lanes and filter as they need.
     *
     * @return array<int, array{id:int, key:string, title:string, estimated_minutes:?int, logged_minutes:?int, lane_id:?int, assignee_id:?int, sprint_id:?int}>
     */
    public function getProjectIssues(int $projectId): array
    {
        $body = $this->request()->get("/api/projects/{$projectId}/issues")->throw()->json();
        $rows = $body['data'] ?? $body;

        return array_map(fn (array $row) => [
            'id' => $row['id'],
            'key' => $row['key'],
            'title' => $row['title'],
            'estimated_minutes' => $row['estimated_minutes'] ?? null,
            'logged_minutes' => $row['logged_minutes'] ?? null,
            'lane_id' => $row['lane_id'] ?? null,
            'assignee_id' => $row['assignee_id'] ?? null,
            'sprint_id' => $row['sprint_id'] ?? null,
        ], $rows);
    }

    /**
     * A project's sprints (minimal mirror for the Client brief, issue #56). Kendo
     * status: 1 = active. Field names are mapped defensively — only `status` is
     * confirmed against live data (activeSprintId).
     *
     * ponytail: name/date keys guessed (name??title, start/end); verify against live API.
     *
     * @return array<int, array{id:int, name:?string, status:?int, starts_at:?string, ends_at:?string}>
     */
    public function getSprints(int $projectId): array
    {
        $body = $this->request()->get("/api/projects/{$projectId}/sprints")->throw()->json();
        $rows = $body['data'] ?? $body;

        return array_map(fn (array $row) => [
            'id' => (int) $row['id'],
            'name' => $row['name'] ?? $row['title'] ?? null,
            'status' => $row['status'] ?? null,
            'starts_at' => $row['starts_at'] ?? $row['start_date'] ?? null,
            'ends_at' => $row['ends_at'] ?? $row['end_date'] ?? null,
        ], $rows);
    }

    /**
     * The whole Kendo user roster (issue #55 / R2). One global call; the id joins
     * to issue assignee_id and worklog user_id. `deleted_at` null = active.
     *
     * @return array<int, array{id:int, name:string, email:?string, active:bool, avatar_url:?string}>
     */
    public function getUsers(): array
    {
        $body = $this->request()->get('/api/users')->throw()->json();
        $rows = $body['data'] ?? $body;

        return array_map(fn (array $row) => [
            'id' => (int) $row['id'],
            'name' => trim(($row['first_name'] ?? '').' '.($row['last_name'] ?? '')),
            'email' => $row['email'] ?? null,
            'active' => ($row['deleted_at'] ?? null) === null,
            'avatar_url' => $row['profile_picture']['webp'] ?? null,
        ], $rows);
    }

    /**
     * A project's lanes as {id, order}. The issue sync uses this to classify each
     * lane (Kendo exposes no done flag — see SyncKendoProjectIssues).
     *
     * @return array<int, array{id:int, title:?string, order:int}>
     */
    public function getProjectLanes(int $projectId): array
    {
        $lanes = $this->request()->get("/api/projects/{$projectId}/lanes")->throw()->json();

        return array_map(fn (array $lane) => [
            'id' => (int) $lane['id'],
            'title' => $lane['title'] ?? null,
            'order' => (int) ($lane['order'] ?? 0),
        ], $lanes);
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
            // Manual entries omit a clock start; created_at (the log date) is the
            // only date they carry, and every reader buckets by started_at.
            'started_at' => $row['started_at'] ?? $row['created_at'],
            'note' => $row['note'] ?? null,
            'issue_key' => $row['issue_key'] ?? null,
            'issue_title' => $row['issue_title'] ?? null,
        ], $rows);
    }

    /** Kendo priority label → wire int (0 Highest … 4 Lowest); null if unknown. */
    public static function priorityToInt(string $label): ?int
    {
        return array_flip(self::PRIORITIES)[$label] ?? null;
    }

    /**
     * Fetch one issue's full Kendo object — the read half of the priority
     * read-modify-write (ADR-0014). Returned as-is (unmapped) so it can be
     * mutated and PUT back without losing unmirrored fields (e.g. description).
     *
     * @return array<string, mixed>
     */
    public function getIssue(int $projectId, int $issueId): array
    {
        return $this->request()
            ->get("/api/projects/{$projectId}/issues/{$issueId}")
            ->throw()
            ->json();
    }

    /**
     * Full-replace update of one issue (no PATCH exists). Caller passes the
     * whole object from getIssue() with fields mutated — reconstructing it from
     * Fylla's partial mirror would clobber unmirrored fields (ADR-0014).
     *
     * @param  array<string, mixed>  $issue
     */
    public function updateIssue(int $projectId, int $issueId, array $issue): void
    {
        $this->request()
            ->put("/api/projects/{$projectId}/issues/{$issueId}", $issue)
            ->throw();
    }

    /**
     * Create a new issue (ADR-0012 promote). Assigned to the caller so it comes
     * back in the my-issues feed and syncs into the local mirror like any other
     * issue, and dropped into the project's first lane + active sprint (Kendo
     * requires lane_id/type/description explicitly on create). A draft carries
     * no description or type, so the title doubles as the description and the
     * type defaults to Task. Returns the new Kendo issue id.
     */
    public function createIssue(int $projectId, string $title, ?int $priority, ?int $assigneeId): int
    {
        $payload = [
            'title' => $title,
            'description' => $title,
            'lane_id' => $this->firstLaneId($projectId),
            'type' => 2, // Task
        ];
        if ($priority !== null) {
            $payload['priority'] = $priority;
        }
        if ($assigneeId !== null) {
            $payload['assignee_id'] = $assigneeId;
        }
        if (($sprintId = $this->activeSprintId($projectId)) !== null) {
            $payload['sprint_id'] = $sprintId;
        }

        return (int) $this->request()
            ->post("/api/projects/{$projectId}/issues", $payload)
            ->throw()
            ->json('id');
    }

    /** First lane (lowest order) of a project — the create default column. */
    private function firstLaneId(int $projectId): int
    {
        $lanes = $this->request()->get("/api/projects/{$projectId}/lanes")->throw()->json();
        usort($lanes, fn (array $a, array $b) => ($a['order'] ?? 0) <=> ($b['order'] ?? 0));

        return (int) $lanes[0]['id'];
    }

    /** The project's active sprint id (status 1), or null if none is running. */
    private function activeSprintId(int $projectId): ?int
    {
        $sprints = $this->request()->get("/api/projects/{$projectId}/sprints")->throw()->json();
        foreach ($sprints as $sprint) {
            if (($sprint['status'] ?? null) === 1) {
                return (int) $sprint['id'];
            }
        }

        return null;
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

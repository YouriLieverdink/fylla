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
}

<?php

namespace Tests\Feature;

use App\ClientContext\ClientContextReport;
use App\Models\Client;
use App\Models\Developer;
use App\Models\Project;
use App\Models\Sprint;
use App\Models\SyncedIssue;
use App\Models\SyncedWorklog;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class ClientContextReportTest extends TestCase
{
    use RefreshDatabase;

    private const NOW = '2026-07-20 12:00:00'; // a Monday

    private const PID = 10;

    private int $seq = 1;

    private Client $client;

    protected function setUp(): void
    {
        parent::setUp();
        $this->client = Client::create(['name' => 'Meridian', 'monthly_target_hours' => 160]);
        Project::create(['kendo_id' => self::PID, 'name' => 'Web', 'client_id' => $this->client->id]);
    }

    private function report(): array
    {
        return (new ClientContextReport(CarbonImmutable::parse(self::NOW)))->generate($this->client->fresh());
    }

    private function issue(array $attrs): SyncedIssue
    {
        return SyncedIssue::create(array_merge([
            'kendo_id' => $this->seq++,
            'key' => 'K-'.$this->seq,
            'title' => 'Issue '.$this->seq,
            'project_id' => self::PID,
            'lane_entered_at' => self::NOW, // recent by default → not stuck
        ], $attrs));
    }

    private function worklog(int $minutes, int $userId, string $at, ?int $issueId = null): void
    {
        SyncedWorklog::create([
            'kendo_worklog_id' => $this->seq++,
            'kendo_user_id' => $userId,
            'kendo_project_id' => self::PID,
            'kendo_issue_id' => $issueId,
            'minutes' => $minutes,
            'started_at' => $at,
        ]);
    }

    /** @return array<string,array<string,mixed>> issues keyed by key */
    private function byKey(array $report): array
    {
        return collect($report['issues'])->keyBy('key')->all();
    }

    public function test_brief_totals_hours_active_and_flagged(): void
    {
        $this->worklog(120, userId: 4, at: '2026-07-05 09:00:00');
        $this->worklog(180, userId: 99, at: '2026-07-06 09:00:00'); // teammate counts
        $this->worklog(600, userId: 4, at: '2026-06-30 09:00:00');  // last month excluded

        $this->issue(['assignee_id' => 4, 'lane_position' => 'middle', 'estimated_minutes' => 60, 'logged_minutes' => 600]); // overrunning
        $this->issue(['assignee_id' => 4, 'lane_position' => 'done', 'estimated_minutes' => 60, 'logged_minutes' => 60]);

        $b = $this->report()['client'];

        $this->assertSame(5, $b['hours']);
        $this->assertSame(3, $b['pct']);
        $this->assertSame(1, $b['activeIssues']);
        $this->assertSame(1, $b['overrunningCount']);
        $this->assertSame(1, $b['flaggedCount']);
        // Pace: 5h over 14 of 23 July weekdays → run-rate ≈ 8h, far under 160h.
        $this->assertSame(8, $b['projected']);
        $this->assertSame(-152, $b['paceDelta']);
    }

    public function test_lane_columns_are_ordered_first_then_middle_alpha_then_done(): void
    {
        $this->issue(['lane_position' => 'done', 'lane_name' => 'Done', 'assignee_id' => 4]);
        $this->issue(['lane_position' => 'middle', 'lane_name' => 'In Review', 'assignee_id' => 4]);
        $this->issue(['lane_position' => 'first', 'lane_name' => 'Backlog', 'assignee_id' => 4]);
        $this->issue(['lane_position' => 'middle', 'lane_name' => 'In Progress', 'assignee_id' => 4]);

        $lanes = $this->report()['lanes'];
        $this->assertSame(['Backlog', 'In Progress', 'In Review', 'Done'], array_column($lanes, 'name'));
        $this->assertSame([false, false, false, true], array_column($lanes, 'done')); // done column marked
    }

    public function test_developer_options_list_assignees_alphabetically_with_month_hours(): void
    {
        Developer::create(['kendo_id' => 4, 'name' => 'Zed', 'email' => null]);
        Developer::create(['kendo_id' => 5, 'name' => 'Amy', 'email' => null]);
        $this->issue(['assignee_id' => 4, 'lane_position' => 'middle']);
        $this->issue(['assignee_id' => 5, 'lane_position' => 'middle']);
        $this->issue(['assignee_id' => null, 'lane_position' => 'middle']); // no phantom option
        $this->worklog(120, userId: 4, at: '2026-07-05 09:00:00');

        $devs = collect($this->report()['developers'])->keyBy('name');
        $this->assertSame(['Amy', 'Zed'], $devs->keys()->all());
        $this->assertSame(2.0, $devs['Zed']['hoursMonth']);
        $this->assertSame(0.0, $devs['Amy']['hoursMonth']);
    }

    public function test_issue_card_carries_flags_names_and_lane(): void
    {
        Developer::create(['kendo_id' => 4, 'name' => 'Sofia Reyes', 'email' => null]);
        $over = $this->issue(['key' => 'OVER', 'assignee_id' => 4, 'lane_position' => 'middle', 'lane_name' => 'In Progress', 'estimated_minutes' => 120, 'logged_minutes' => 360]);
        $this->issue(['key' => 'UNASSIGNED', 'assignee_id' => null, 'lane_position' => 'middle', 'estimated_minutes' => 120, 'logged_minutes' => 60]);

        $cards = $this->byKey($this->report());

        $this->assertTrue($cards['OVER']['over']);
        $this->assertSame(200, $cards['OVER']['overPct']);
        $this->assertSame('Sofia Reyes', $cards['OVER']['assigneeName']);
        $this->assertSame('In Progress', $cards['OVER']['lane']);
        $this->assertFalse($cards['UNASSIGNED']['over']);
        $this->assertSame('Unassigned', $cards['UNASSIGNED']['assigneeName']);
    }

    public function test_stuck_only_flags_inactive_in_flight_issues(): void
    {
        $this->issue(['key' => 'FRESH', 'assignee_id' => 4, 'lane_position' => 'middle', 'lane_entered_at' => '2026-07-17 09:00:00']); // recent → not stuck
        $this->issue(['key' => 'STALE', 'assignee_id' => 4, 'lane_position' => 'middle', 'lane_entered_at' => '2026-07-01 09:00:00']); // old, no work → stuck
        $this->issue(['key' => 'DONE', 'assignee_id' => 4, 'lane_position' => 'done', 'lane_entered_at' => '2026-07-01 09:00:00']);   // done → never stuck

        $cards = $this->byKey($this->report());

        $this->assertFalse($cards['FRESH']['stuck']);
        $this->assertTrue($cards['STALE']['stuck']);
        $this->assertFalse($cards['DONE']['stuck']);
    }

    public function test_recent_worklog_keeps_an_old_lane_issue_unstuck(): void
    {
        $i = $this->issue(['key' => 'REVIVED', 'assignee_id' => 4, 'lane_position' => 'middle', 'lane_entered_at' => '2026-07-01 09:00:00']);
        $this->worklog(60, userId: 4, at: '2026-07-17 09:00:00', issueId: $i->kendo_id);

        $this->assertFalse($this->byKey($this->report())['REVIVED']['stuck']);
    }

    public function test_current_sprint_reports_done_over_total(): void
    {
        Sprint::create(['kendo_id' => 50, 'project_id' => self::PID, 'name' => 'Sprint 24', 'status' => 1, 'starts_at' => '2026-07-15', 'ends_at' => '2026-07-28']);
        $this->issue(['lane_position' => 'done', 'sprint_id' => 50, 'assignee_id' => 4]);
        $this->issue(['lane_position' => 'middle', 'sprint_id' => 50, 'assignee_id' => 4]);

        $sprint = $this->report()['client']['sprint'];

        $this->assertSame('Sprint 24', $sprint['name']);
        $this->assertSame(1, $sprint['done']);
        $this->assertSame(2, $sprint['total']);
        $this->assertSame(8, $sprint['daysLeft']);
    }
}

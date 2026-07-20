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

    private const NOW = '2026-07-20 12:00:00';

    private const PID = 10; // the client's one project (kendo id)

    private int $kendoId = 1;

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
            'kendo_id' => $this->kendoId++,
            'key' => 'K-'.$this->kendoId,
            'title' => 'Issue '.$this->kendoId,
            'project_id' => self::PID,
        ], $attrs));
    }

    private function worklog(int $minutes, int $userId, string $at): void
    {
        SyncedWorklog::create([
            'kendo_worklog_id' => $this->kendoId++,
            'kendo_user_id' => $userId,
            'kendo_project_id' => self::PID,
            'kendo_issue_id' => 1,
            'minutes' => $minutes,
            'started_at' => $at,
        ]);
    }

    public function test_brief_sums_team_hours_this_month_against_target(): void
    {
        $this->worklog(120, userId: 4, at: '2026-07-05 09:00:00');  // me
        $this->worklog(180, userId: 99, at: '2026-07-06 09:00:00'); // teammate — still counted
        $this->worklog(600, userId: 4, at: '2026-06-30 09:00:00');  // last month — excluded

        $brief = $this->report()['client'];

        $this->assertSame(5, $brief['hours']); // (120+180)/60
        $this->assertSame(160, $brief['target']);
        $this->assertSame(3, $brief['pct']); // 5/160
    }

    public function test_active_issues_and_needs_attention_counts(): void
    {
        $this->issue(['assignee_id' => 4, 'lane_position' => 'done', 'estimated_minutes' => 60, 'logged_minutes' => 60]);
        $this->issue(['assignee_id' => 4, 'lane_position' => 'middle', 'estimated_minutes' => 60, 'logged_minutes' => 600, 'lane_entered_at' => '2026-07-18 09:00:00']); // overrunning + aging
        $this->issue(['assignee_id' => 4, 'lane_position' => 'first', 'estimated_minutes' => 60, 'logged_minutes' => 30]);

        $brief = $this->report()['client'];

        $this->assertSame(2, $brief['activeIssues']);      // middle + first (done excluded)
        $this->assertSame(1, $brief['overrunningCount']);  // the middle one
        $this->assertSame(1, $brief['agingCount']);        // the middle one
    }

    public function test_current_sprint_reports_done_over_total(): void
    {
        Sprint::create(['kendo_id' => 50, 'project_id' => self::PID, 'name' => 'Sprint 24', 'status' => 1, 'starts_at' => '2026-07-15', 'ends_at' => '2026-07-28']);
        Sprint::create(['kendo_id' => 51, 'project_id' => self::PID, 'name' => 'Old', 'status' => 2, 'ends_at' => '2026-07-01']); // not active
        $this->issue(['assignee_id' => 4, 'lane_position' => 'done', 'sprint_id' => 50, 'estimated_minutes' => 60, 'logged_minutes' => 60]);
        $this->issue(['assignee_id' => 4, 'lane_position' => 'middle', 'sprint_id' => 50, 'estimated_minutes' => 60, 'logged_minutes' => 60]);

        $sprint = $this->report()['client']['sprint'];

        $this->assertSame('Sprint 24', $sprint['name']);
        $this->assertSame(1, $sprint['done']);
        $this->assertSame(2, $sprint['total']);
        $this->assertSame(8, $sprint['daysLeft']); // Jul 20 → Jul 28
    }

    public function test_no_active_sprint_is_null(): void
    {
        $this->assertNull($this->report()['client']['sprint']);
    }

    public function test_per_developer_median_bias_and_within(): void
    {
        Developer::create(['kendo_id' => 7, 'name' => 'Sofia Reyes', 'email' => 's@x.io']);
        // est 4h/6h, logged 4h/10h → sum est 10h, logged 14h → +40% bias.
        $this->issue(['assignee_id' => 7, 'lane_position' => 'done', 'estimated_minutes' => 240, 'logged_minutes' => 240, 'lane_entered_at' => '2026-07-10 09:00:00']);
        $this->issue(['assignee_id' => 7, 'lane_position' => 'done', 'estimated_minutes' => 360, 'logged_minutes' => 600, 'lane_entered_at' => '2026-07-11 09:00:00']);

        $dev = $this->report()['developers'][0];

        $this->assertSame('Sofia Reyes', $dev['name']);
        $this->assertSame(40, $dev['biasPct']);
        $this->assertSame(5.0, $dev['medianEst']);    // median(240,360) = 300min
        $this->assertSame(7.0, $dev['medianActual']); // median(240,600) = 420min
        $this->assertSame(50, $dev['withinPct']);     // issue1 within, issue2 not
        $this->assertSame(2, $dev['sample']);
    }

    public function test_developers_without_estimated_done_issues_sit_out(): void
    {
        // Only open work, and only estimateless done work → no bias to show.
        $this->issue(['assignee_id' => 7, 'lane_position' => 'middle', 'estimated_minutes' => 60, 'logged_minutes' => 60]);
        $this->issue(['assignee_id' => 8, 'lane_position' => 'done', 'estimated_minutes' => 0, 'logged_minutes' => 120]);

        $this->assertCount(0, $this->report()['developers']);
    }

    public function test_rolling_window_caps_at_20_newest_by_lane_entered_at(): void
    {
        // 20 accurate issues (bias 0), plus one older wildly-overrun issue that
        // the cap must drop (ordering is lane_entered_at desc).
        for ($d = 1; $d <= 20; $d++) {
            $this->issue(['assignee_id' => 7, 'lane_position' => 'done', 'estimated_minutes' => 60, 'logged_minutes' => 60, 'lane_entered_at' => sprintf('2026-07-%02d 09:00:00', $d)]);
        }
        $this->issue(['assignee_id' => 7, 'lane_position' => 'done', 'estimated_minutes' => 60, 'logged_minutes' => 6000, 'lane_entered_at' => '2026-06-01 09:00:00']);

        $dev = $this->report()['developers'][0];

        $this->assertSame(20, $dev['sample']);
        $this->assertSame(0, $dev['biasPct']); // the +9900% outlier is outside the window
    }

    public function test_overrunning_lists_in_flight_over_budget_worst_first(): void
    {
        Developer::create(['kendo_id' => 7, 'name' => 'Sofia', 'email' => 's@x.io']);
        $this->issue(['assignee_id' => 7, 'lane_position' => 'middle', 'estimated_minutes' => 60, 'logged_minutes' => 90, 'lane_entered_at' => self::NOW]);  // +50%
        $this->issue(['assignee_id' => 7, 'lane_position' => 'middle', 'estimated_minutes' => 60, 'logged_minutes' => 180, 'lane_entered_at' => self::NOW]); // +200%
        $this->issue(['assignee_id' => 7, 'lane_position' => 'done', 'estimated_minutes' => 60, 'logged_minutes' => 600]);   // done → excluded
        $this->issue(['assignee_id' => 7, 'lane_position' => 'middle', 'estimated_minutes' => 60, 'logged_minutes' => 30, 'lane_entered_at' => self::NOW]);  // under → excluded

        $over = $this->report()['overrunning'];

        $this->assertCount(2, $over);
        $this->assertSame(200, $over[0]['overPct']); // worst first
        $this->assertSame(50, $over[1]['overPct']);
    }

    public function test_aging_lists_middle_lane_longest_first_with_days_and_lane(): void
    {
        $this->issue(['assignee_id' => 7, 'lane_position' => 'middle', 'lane_name' => 'In review', 'estimated_minutes' => 60, 'logged_minutes' => 30, 'lane_entered_at' => '2026-07-14 12:00:00']); // 6 days
        $this->issue(['assignee_id' => 7, 'lane_position' => 'middle', 'lane_name' => 'In progress', 'estimated_minutes' => 60, 'logged_minutes' => 30, 'lane_entered_at' => '2026-07-18 12:00:00']); // 2 days
        $this->issue(['assignee_id' => 7, 'lane_position' => 'done', 'estimated_minutes' => 60, 'logged_minutes' => 30]); // excluded

        $aging = $this->report()['aging'];

        $this->assertCount(2, $aging);
        $this->assertSame(6, $aging[0]['days']); // longest in lane first
        $this->assertSame('In review', $aging[0]['lane']);
        $this->assertSame(2, $aging[1]['days']);
    }
}

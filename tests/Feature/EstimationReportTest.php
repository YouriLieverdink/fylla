<?php

namespace Tests\Feature;

use App\Estimation\EstimationReport;
use App\Models\FinishedIssue;
use App\Models\Project;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class EstimationReportTest extends TestCase
{
    use RefreshDatabase;

    private int $kendoId = 1;

    private function finished(int $estimateMinutes, int $loggedMinutes, int $projectId = 1, ?string $lastWorked = '2026-07-10 09:00:00'): FinishedIssue
    {
        return FinishedIssue::create([
            'kendo_id' => $this->kendoId++,
            'key' => 'K-'.$this->kendoId,
            'title' => 'Issue '.$this->kendoId,
            'project_id' => $projectId,
            'estimated_minutes' => $estimateMinutes,
            'logged_minutes' => $loggedMinutes,
            'last_worked_at' => $lastWorked,
            'synced_at' => '2026-07-16 12:00:00',
        ]);
    }

    public function test_bias_is_the_summed_actual_over_estimate_across_finished_issues(): void
    {
        // Estimated 10h total, logged 14h total → +40% (underestimates by 40%).
        $this->finished(estimateMinutes: 240, loggedMinutes: 240);  // est 4h, actual 4h
        $this->finished(estimateMinutes: 360, loggedMinutes: 600);  // est 6h, actual 10h

        $report = (new EstimationReport)->generate();

        $this->assertSame(40, $report['bias']['pct']);
        $this->assertSame(2, $report['bias']['sampleSize']);
        $this->assertSame(10.0, $report['bias']['estimateHours']);
        $this->assertSame(14.0, $report['bias']['actualHours']);
    }

    public function test_estimateless_issues_list_but_sit_out_the_bias(): void
    {
        $this->finished(estimateMinutes: 0, loggedMinutes: 300);   // no estimate
        $this->finished(estimateMinutes: 120, loggedMinutes: 120); // the only contributor

        $report = (new EstimationReport)->generate();

        $this->assertCount(2, $report['issues']);
        $this->assertSame(1, $report['bias']['sampleSize']);
        $this->assertSame(0, $report['bias']['pct']); // 2h logged vs 2h estimate
    }

    public function test_per_issue_rows_carry_estimate_actual_and_bias(): void
    {
        $this->finished(estimateMinutes: 120, loggedMinutes: 180); // 2h est, 3h actual

        $row = (new EstimationReport)->generate()['issues'][0];

        $this->assertSame(2.0, $row['estimateHours']);
        $this->assertSame(3.0, $row['actualHours']);
        $this->assertSame(50, $row['biasPct']);
    }

    public function test_estimateless_row_reports_null_bias(): void
    {
        $this->finished(estimateMinutes: 0, loggedMinutes: 180);

        $row = (new EstimationReport)->generate()['issues'][0];

        $this->assertNull($row['estimateHours']);
        $this->assertNull($row['biasPct']);
        $this->assertSame(3.0, $row['actualHours']);
    }

    public function test_rows_order_most_recently_worked_first_nulls_last(): void
    {
        $this->finished(estimateMinutes: 60, loggedMinutes: 60, lastWorked: '2026-07-01 09:00:00');
        $this->finished(estimateMinutes: 60, loggedMinutes: 60, lastWorked: null);
        $recent = $this->finished(estimateMinutes: 60, loggedMinutes: 60, lastWorked: '2026-07-14 09:00:00');

        $keys = array_column((new EstimationReport)->generate()['issues'], 'key');

        $this->assertSame($recent->key, $keys[0]);   // newest first
        $this->assertNull((new EstimationReport)->generate()['issues'][2]['lastWorked']); // null last
    }

    public function test_slice_by_project_narrows_bias_and_rows(): void
    {
        Project::create(['kendo_id' => 1, 'name' => 'Alpha']);
        Project::create(['kendo_id' => 2, 'name' => 'Beta']);

        $this->finished(estimateMinutes: 60, loggedMinutes: 120, projectId: 1); // +100%
        $this->finished(estimateMinutes: 60, loggedMinutes: 60, projectId: 2);  // 0%

        $sliced = (new EstimationReport)->generate([1]);

        $this->assertCount(1, $sliced['issues']);
        $this->assertSame('Alpha', $sliced['issues'][0]['project']);
        $this->assertSame(100, $sliced['bias']['pct']);
        // Options list every project with finished work, regardless of the slice.
        $this->assertSame(['Alpha', 'Beta'], array_column($sliced['projects'], 'name'));
    }

    public function test_no_finished_issues_yields_empty_report(): void
    {
        $report = (new EstimationReport)->generate();

        $this->assertSame([], $report['issues']);
        $this->assertNull($report['bias']['pct']);
        $this->assertSame(0, $report['bias']['sampleSize']);
    }
}

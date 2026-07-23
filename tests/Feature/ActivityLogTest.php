<?php

namespace Tests\Feature;

use App\Jobs\PostWorklog;
use App\Listeners\JobRunRecorder;
use App\Models\Issue;
use App\Models\JobRun;
use App\Models\Worklog;
use Illuminate\Contracts\Queue\Job;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Http\Client\RequestException;
use Illuminate\Queue\Events\JobFailed;
use Illuminate\Queue\Events\JobProcessed;
use Illuminate\Queue\Events\JobProcessing;
use Illuminate\Support\Facades\Http;
use Inertia\Testing\AssertableInertia;
use Mockery;
use RuntimeException;
use Tests\TestCase;

/**
 * Job & Sync Activity Log backbone (#87): the queue-event capture writes one
 * `job_runs` row per run and reconciles its status, and `/activity` renders the
 * flat list. Capture is exercised through the real listeners via dispatchSync —
 * the sync path fires the same JobProcessing/Processed/Failed events as the
 * queued and manual paths (#83).
 */
class ActivityLogTest extends TestCase
{
    use RefreshDatabase;

    private function worklog(array $overrides = []): Worklog
    {
        $issue = Issue::create(['kendo_id' => 42, 'key' => 'X-1', 'title' => 'X', 'project_id' => 7]);

        return Worklog::create(array_merge([
            'issue_id' => $issue->id,
            'timer_id' => $issue->timers()->create()->id,
            'kendo_project_id' => 7,
            'kendo_issue_id' => 42,
            'minutes' => 30,
            'started_at' => now(),
            'comment' => 'did stuff',
        ], $overrides));
    }

    /** A queue Job stub carrying just the three fields the recorder reads. */
    private function fakeJob(string $uuid, string $class): Job
    {
        $job = Mockery::mock(Job::class);
        $job->shouldReceive('uuid')->andReturn($uuid);
        $job->shouldReceive('resolveName')->andReturn($class);
        $job->shouldReceive('attempts')->andReturn(1);

        return $job;
    }

    public function test_processing_writes_a_running_row_that_processed_flips_to_ok(): void
    {
        $recorder = new JobRunRecorder;
        $job = $this->fakeJob('u1', 'App\Jobs\SyncKendoIssues');

        $recorder->processing(new JobProcessing('sync', $job));
        $this->assertSame('running', JobRun::sole()->status);
        $this->assertNull(JobRun::sole()->finished_at);

        $recorder->processed(new JobProcessed('sync', $job));
        $run = JobRun::sole(); // same row, upserted on uuid
        $this->assertSame('ok', $run->status);
        $this->assertNotNull($run->finished_at);
    }

    public function test_processing_then_failed_flips_the_same_row_to_failed(): void
    {
        $recorder = new JobRunRecorder;
        $job = $this->fakeJob('u2', 'App\Jobs\SyncKendoIssues');

        $recorder->processing(new JobProcessing('sync', $job));
        $recorder->failed(new JobFailed('sync', $job, new RuntimeException('Kendo 502')));

        $run = JobRun::sole();
        $this->assertSame('failed', $run->status);
        $this->assertSame('Kendo 502', $run->error);
    }

    public function test_a_successful_run_is_recorded_running_then_ok(): void
    {
        Http::fake(['*/time-entries' => Http::response(['id' => 999], 201)]);

        PostWorklog::dispatchSync($this->worklog());

        $run = JobRun::sole();
        $this->assertSame('ok', $run->status);
        $this->assertSame(PostWorklog::class, $run->job_class);
        $this->assertSame('worklog-post', $run->trigger);
        $this->assertNull($run->moment_id);
        $this->assertNotNull($run->started_at);
        $this->assertNotNull($run->finished_at);
        $this->assertGreaterThanOrEqual(1, $run->attempts);
    }

    public function test_a_failing_run_is_recorded_failed_with_the_error_message(): void
    {
        Http::fake(['*/time-entries' => Http::response('boom', 500)]);

        try {
            PostWorklog::dispatchSync($this->worklog());
            $this->fail('expected the job to throw');
        } catch (RequestException) {
            // SyncQueue rethrows after firing JobFailed — the recorder already ran.
        }

        $run = JobRun::sole();
        $this->assertSame('failed', $run->status);
        $this->assertNotNull($run->error);
        $this->assertNotNull($run->finished_at);
    }

    public function test_manual_sync_records_every_job_under_one_shared_moment(): void
    {
        // Catch-all: the jobs' provider shapes don't matter here — a run is
        // recorded whether the call succeeds or fails, and either way it carries
        // the manual trigger and the one moment_id minted by the controller.
        Http::fake(['*' => Http::response([], 200)]);
        config(['fylla.github_pr_queries' => []]);

        $this->post('/sync')->assertRedirect();

        $runs = JobRun::all();
        $this->assertGreaterThan(1, $runs->count());
        $this->assertSame(1, $runs->pluck('moment_id')->unique()->count());
        $this->assertNotNull($runs->first()->moment_id);
        $this->assertTrue($runs->every(fn (JobRun $r) => $r->trigger === 'manual'));
    }

    public function test_activity_page_renders_a_flat_list_newest_first(): void
    {
        JobRun::create([
            'uuid' => 'a', 'job_class' => 'App\Jobs\SyncKendoIssues', 'trigger' => 'scheduled',
            'status' => 'ok', 'started_at' => '2026-07-23 09:00:00', 'finished_at' => '2026-07-23 09:00:01',
        ]);
        JobRun::create([
            'uuid' => 'b', 'job_class' => 'App\Jobs\PostWorklog', 'trigger' => 'worklog-post',
            'status' => 'failed', 'started_at' => '2026-07-23 10:00:00', 'error' => 'Kendo 502',
        ]);

        $this->get('/activity')
            ->assertOk()
            ->assertInertia(fn (AssertableInertia $page) => $page
                ->component('Activity')
                ->has('runs', 2)
                ->where('runs.0.jobClass', 'PostWorklog')
                ->where('runs.0.status', 'failed')
                ->where('runs.0.error', 'Kendo 502')
                ->where('runs.1.jobClass', 'SyncKendoIssues'));
    }
}

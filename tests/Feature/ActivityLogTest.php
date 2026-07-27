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
use Illuminate\Support\Facades\Queue;
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

    /**
     * Post a worklog against a failing provider and return the failed run row
     * it recorded — the starting point for every retry case.
     */
    private function failedRun(Worklog $worklog): JobRun
    {
        try {
            PostWorklog::dispatchSync($worklog);
            $this->fail('expected the job to throw');
        } catch (RequestException) {
            // SyncQueue rethrows after firing JobFailed — the run is recorded.
        }

        return JobRun::where('worklog_id', $worklog->id)->sole();
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

    public function test_activity_page_groups_runs_by_moment_newest_first(): void
    {
        // Pinned so the fixed timestamps below stay inside the header's
        // one-day failure window.
        $this->travelTo('2026-07-23 12:00:00');

        // One sync moment (two jobs, one failed) + one standalone worklog post.
        JobRun::create([
            'uuid' => 'a', 'moment_id' => 'moment-1', 'job_class' => 'App\Jobs\SyncKendoIssues',
            'trigger' => 'manual', 'status' => 'ok',
            'started_at' => '2026-07-23 09:00:00', 'finished_at' => '2026-07-23 09:00:01',
        ]);
        JobRun::create([
            'uuid' => 'b', 'moment_id' => 'moment-1', 'job_class' => 'App\Jobs\SyncKendoWorklogs',
            'trigger' => 'manual', 'status' => 'failed',
            'started_at' => '2026-07-23 09:00:02', 'error' => 'Kendo 502',
        ]);
        JobRun::create([
            'uuid' => 'c', 'moment_id' => null, 'job_class' => 'App\Jobs\PostWorklog',
            'trigger' => 'worklog-post', 'status' => 'ok',
            'started_at' => '2026-07-23 10:00:00', 'finished_at' => '2026-07-23 10:00:01',
        ]);

        $this->get('/activity')
            ->assertOk()
            ->assertInertia(fn (AssertableInertia $page) => $page
                ->component('Activity')
                ->has('moments', 2)
                // Newest first: the standalone worklog post leads, its own group.
                ->where('moments.0.trigger', 'worklog-post')
                ->where('moments.0.status', 'ok')
                ->has('moments.0.runs', 1)
                // The sync moment rolls its failed child up to the moment level.
                ->where('moments.1.trigger', 'manual')
                ->where('moments.1.status', 'failed')
                ->where('moments.1.failedCount', 1)
                ->has('moments.1.runs', 2)
                // Recent failure surfaces on the shared header signal.
                ->where('activityFailures', 1));
    }

    public function test_a_moment_with_a_running_child_and_no_failure_rolls_up_to_running(): void
    {
        JobRun::create([
            'uuid' => 'r1', 'moment_id' => 'moment-2', 'job_class' => 'App\Jobs\SyncKendoIssues',
            'trigger' => 'scheduled', 'status' => 'ok',
            'started_at' => '2026-07-23 11:00:00', 'finished_at' => '2026-07-23 11:00:01',
        ]);
        JobRun::create([
            'uuid' => 'r2', 'moment_id' => 'moment-2', 'job_class' => 'App\Jobs\SyncKendoWorklogs',
            'trigger' => 'scheduled', 'status' => 'running', 'started_at' => '2026-07-23 11:00:02',
        ]);

        $this->get('/activity')
            ->assertInertia(fn (AssertableInertia $page) => $page
                ->where('moments.0.status', 'running')
                ->where('moments.0.failedCount', 0));
    }

    public function test_activity_failures_signal_ignores_stale_failures(): void
    {
        JobRun::create([
            'uuid' => 'old', 'job_class' => 'App\Jobs\SyncKendoIssues', 'trigger' => 'scheduled',
            'status' => 'failed', 'started_at' => now()->subDays(3), 'error' => 'old boom',
        ]);

        $this->get('/activity')
            ->assertInertia(fn (AssertableInertia $page) => $page->where('activityFailures', 0));
    }

    /**
     * Worklog retry (#89). The retry handle is the run's worklog_id, stamped by
     * the job itself, so every dispatch site carries it.
     */
    public function test_a_failed_worklog_post_is_retryable_and_a_failed_sync_is_not(): void
    {
        Http::fake(['*/time-entries' => Http::response('boom', 500)]);
        $worklog = $this->worklog();
        $this->failedRun($worklog);

        JobRun::create([
            'uuid' => 'sync-fail', 'job_class' => 'App\Jobs\SyncKendoIssues', 'trigger' => 'scheduled',
            'status' => 'failed', 'started_at' => now()->subMinute(), 'error' => 'Kendo 502',
        ]);

        $this->get('/activity')
            ->assertInertia(fn (AssertableInertia $page) => $page
                // Newest first: the worklog post leads, the sync failure follows.
                ->where('moments.0.runs.0.canRetry', true)
                ->where('moments.1.runs.0.canRetry', false));
    }

    /**
     * The retry must be *queued*, not inline (#86) — the endpoint returns
     * before the post is attempted. Asserted under Queue::fake because the
     * suite's sync driver would otherwise hide a dispatchSync regression.
     */
    public function test_retry_dispatches_the_job_queued_rather_than_inline(): void
    {
        Http::fake(['*/time-entries' => Http::response('boom', 500)]);
        $failed = $this->failedRun($this->worklog());

        Queue::fake();
        $this->post("/activity/runs/{$failed->id}/retry")->assertRedirect();

        Queue::assertPushed(PostWorklog::class);
    }

    public function test_retry_redispatches_the_worklog_and_writes_a_new_ok_run(): void
    {
        Http::fake(['*/time-entries' => Http::sequence()->push('boom', 500)->push(['id' => 999], 201)]);
        $worklog = $this->worklog();
        $failed = $this->failedRun($worklog);

        $this->post("/activity/runs/{$failed->id}/retry")->assertRedirect();

        // The original row is a permanent record — status, error and timing all
        // survive the retry untouched.
        $this->assertEquals($failed->getAttributes(), $failed->fresh()->getAttributes());

        $retry = JobRun::whereKeyNot($failed->id)->sole();
        $this->assertSame('ok', $retry->status);
        $this->assertSame($worklog->id, $retry->worklog_id);
        $this->assertNotNull($worklog->fresh()->posted_at);
    }

    public function test_retrying_an_already_posted_worklog_makes_no_provider_call(): void
    {
        Http::fake(['*/time-entries' => Http::response('boom', 500)]);
        $worklog = $this->worklog();
        $failed = $this->failedRun($worklog);

        // Posted out-of-band in the interim — the retry must do no HTTP.
        $worklog->update(['posted_at' => now(), 'kendo_worklog_id' => '999']);
        Http::fake(['*' => Http::response([], 500)]);

        $this->post("/activity/runs/{$failed->id}/retry")->assertRedirect();

        Http::assertNothingSent();
        $this->assertSame('ok', JobRun::whereKeyNot($failed->id)->sole()->status);
    }

    public function test_retry_is_rejected_for_a_run_that_is_not_a_failed_worklog_post(): void
    {
        $run = JobRun::create([
            'uuid' => 'sync-fail', 'job_class' => 'App\Jobs\SyncKendoIssues', 'trigger' => 'scheduled',
            'status' => 'failed', 'started_at' => now(), 'error' => 'Kendo 502',
        ]);

        $this->post("/activity/runs/{$run->id}/retry")->assertNotFound();
    }
}

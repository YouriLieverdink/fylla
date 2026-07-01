<?php

namespace Tests\Feature;

use App\Jobs\PostWorklog;
use App\Kendo\Client as KendoClient;
use App\Models\Issue;
use App\Models\Worklog;
use App\Services\TimerService;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Http\Client\RequestException;
use Illuminate\Support\Facades\Bus;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class PostWorklogTest extends TestCase
{
    use RefreshDatabase;

    private function issue(): Issue
    {
        return Issue::create([
            'kendo_id' => 42, 'key' => 'X-1', 'title' => 'X', 'project_id' => 7,
        ]);
    }

    private function worklog(Issue $issue, array $overrides = []): Worklog
    {
        return Worklog::create(array_merge([
            'issue_id' => $issue->id,
            'timer_id' => $issue->timers()->create()->id,
            'minutes' => 30,
            'started_at' => now(),
            'comment' => 'did stuff',
        ], $overrides));
    }

    public function test_closing_a_segment_dispatches_the_job(): void
    {
        Bus::fake();
        $issue = $this->issue();

        app(TimerService::class)->start($issue);
        $this->travel(60)->seconds();
        app(TimerService::class)->pause();

        Bus::assertDispatched(PostWorklog::class);
    }

    public function test_job_posts_payload_and_writes_back_kendo_id(): void
    {
        Http::fake(['*/time-entries' => Http::response(['id' => 999], 201)]);
        $worklog = $this->worklog($this->issue());

        (new PostWorklog($worklog))->handle(app(KendoClient::class));

        Http::assertSent(fn ($req) => str_contains($req->url(), '/api/projects/7/issues/42/time-entries')
            && $req['minutes_spent'] === 30
            && $req['note'] === 'did stuff'
            && ! empty($req['started_at']));

        $worklog->refresh();
        $this->assertSame('999', $worklog->kendo_worklog_id);
        $this->assertNotNull($worklog->posted_at);
    }

    public function test_already_posted_worklog_makes_no_http_call(): void
    {
        Http::fake();
        $worklog = $this->worklog($this->issue(), ['posted_at' => now()]);

        (new PostWorklog($worklog))->handle(app(KendoClient::class));

        Http::assertNothingSent();
    }

    public function test_failure_records_post_error_and_leaves_unposted(): void
    {
        Http::fake(['*/time-entries' => Http::response('boom', 500)]);
        $worklog = $this->worklog($this->issue());
        $job = new PostWorklog($worklog);

        try {
            $job->handle(app(KendoClient::class));
            $this->fail('expected RequestException');
        } catch (RequestException $e) {
            $job->failed($e);
        }

        $worklog->refresh();
        $this->assertNull($worklog->posted_at);
        $this->assertNotNull($worklog->post_error);
    }
}

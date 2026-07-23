<?php

namespace App\Providers;

use App\GitHub\Client as GitHubClient;
use App\Kendo\Client as KendoClient;
use App\Listeners\JobRunRecorder;
use Illuminate\Http\Client\Factory as HttpFactory;
use Illuminate\Queue\Events\JobFailed;
use Illuminate\Queue\Events\JobProcessed;
use Illuminate\Queue\Events\JobProcessing;
use Illuminate\Support\Facades\Event;
use Illuminate\Support\ServiceProvider;

class AppServiceProvider extends ServiceProvider
{
    /**
     * Register any application services.
     */
    public function register(): void
    {
        $this->app->singleton(KendoClient::class, fn ($app) => new KendoClient(
            $app->make(HttpFactory::class),
            (string) config('services.kendo.base_url'),
            (string) config('services.kendo.token'),
        ));

        $this->app->singleton(GitHubClient::class, fn ($app) => new GitHubClient(
            $app->make(HttpFactory::class),
            (string) config('services.github.token'),
        ));
    }

    /**
     * Bootstrap any application services.
     */
    public function boot(): void
    {
        // Activity Log capture (#87): one listener set records sync, queued and
        // manual runs into `job_runs`. Registered after the framework's Context
        // hydration listener (a base provider), so moment_id/trigger are present.
        Event::listen(JobProcessing::class, [JobRunRecorder::class, 'processing']);
        Event::listen(JobProcessed::class, [JobRunRecorder::class, 'processed']);
        Event::listen(JobFailed::class, [JobRunRecorder::class, 'failed']);
    }
}

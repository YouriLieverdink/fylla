<?php

namespace App\Providers;

use App\GitHub\Client as GitHubClient;
use App\Kendo\Client as KendoClient;
use Illuminate\Http\Client\Factory as HttpFactory;
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
        //
    }
}

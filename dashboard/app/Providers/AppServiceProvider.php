<?php

namespace App\Providers;

use Illuminate\Foundation\Console\ServeCommand;
use Illuminate\Support\ServiceProvider;

class AppServiceProvider extends ServiceProvider
{
    /**
     * Register any application services.
     */
    public function register(): void
    {
        //
    }

    /**
     * Bootstrap any application services.
     */
    public function boot(): void
    {
        ServeCommand::$passthroughVariables[] = 'CREEL_REST_ENDPOINT';
        ServeCommand::$passthroughVariables[] = 'CREEL_API_KEY';
        ServeCommand::$passthroughVariables[] = 'DASHBOARD_USERNAME';
        ServeCommand::$passthroughVariables[] = 'DASHBOARD_PASSWORD';
    }
}

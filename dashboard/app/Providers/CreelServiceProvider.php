<?php

namespace App\Providers;

use App\Services\CreelApiClient;
use Illuminate\Support\ServiceProvider;

class CreelServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->app->singleton(CreelApiClient::class);
    }
}

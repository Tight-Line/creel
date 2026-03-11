<?php

namespace Tests\Feature;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Mockery;
use Tests\TestCase;

class DashboardControllerTest extends TestCase
{
    private $api;

    protected function setUp(): void
    {
        parent::setUp();
        $this->api = Mockery::mock(CreelApiClient::class);
        $this->app->instance(CreelApiClient::class, $this->api);
    }

    private function authed()
    {
        return $this->withSession(['authenticated' => true]);
    }

    public function test_index_displays_health_and_counts(): void
    {
        $this->api->shouldReceive('health')->once()->andReturn(['status' => 'ok']);
        $this->api->shouldReceive('getStats')->once()->andReturn([
            'api_key_configs' => 2,
            'llm_configs' => 1,
            'embedding_configs' => 0,
            'extraction_prompt_configs' => 3,
            'topics' => 1,
            'system_accounts' => 0,
            'documents' => 5,
            'chunks' => 42,
            'memories' => 7,
        ]);

        $response = $this->authed()->get('/');

        $response->assertStatus(200);
        $response->assertViewIs('dashboard.index');
        $response->assertViewHas('health', ['status' => 'ok']);
        $response->assertViewHas('counts', [
            'api_key_configs' => 2,
            'llm_configs' => 1,
            'embedding_configs' => 0,
            'extraction_prompt_configs' => 3,
            'topics' => 1,
            'system_accounts' => 0,
            'documents' => 5,
            'chunks' => 42,
            'memories' => 7,
        ]);
    }

    public function test_index_handles_api_error_gracefully(): void
    {
        $this->api->shouldReceive('health')->andThrow(new CreelApiException(500, '', 'Connection failed'));

        $response = $this->authed()->get('/');

        $response->assertStatus(200);
        $response->assertSessionHas('error', 'Connection failed');
        $response->assertViewHas('counts', [
            'api_key_configs' => 0,
            'llm_configs' => 0,
            'embedding_configs' => 0,
            'extraction_prompt_configs' => 0,
            'topics' => 0,
            'system_accounts' => 0,
            'documents' => 0,
            'chunks' => 0,
            'memories' => 0,
        ]);
    }
}

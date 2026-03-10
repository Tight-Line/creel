<?php

namespace Tests\Feature;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Mockery;
use Tests\TestCase;

class MemoryControllerTest extends TestCase
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

    public function test_index_lists_scopes(): void
    {
        $scopes = ['default', 'project-alpha'];
        $this->api->shouldReceive('listMemoryScopes')->once()->andReturn($scopes);

        $response = $this->authed()->get('/memories');

        $response->assertStatus(200);
        $response->assertViewIs('memories.index');
        $response->assertViewHas('scopes', $scopes);
    }

    public function test_index_handles_api_error(): void
    {
        $this->api->shouldReceive('listMemoryScopes')
            ->andThrow(new CreelApiException(500, '', 'Error'));

        $response = $this->authed()->get('/memories');

        $response->assertStatus(200);
        $response->assertSessionHas('error');
        $response->assertViewHas('scopes', []);
    }

    public function test_scope_lists_memories(): void
    {
        $memories = [
            ['id' => 'm1', 'content' => 'User prefers dark mode', 'status' => 'active'],
            ['id' => 'm2', 'content' => 'User likes Go', 'status' => 'active'],
        ];
        $this->api->shouldReceive('listMemories')->with('default')->once()->andReturn($memories);

        $response = $this->authed()->get('/memories/default');

        $response->assertStatus(200);
        $response->assertViewIs('memories.scope');
        $response->assertViewHas('scope', 'default');
        $response->assertViewHas('memories', $memories);
    }

    public function test_scope_redirects_on_api_error(): void
    {
        $this->api->shouldReceive('listMemories')
            ->andThrow(new CreelApiException(404, '', 'Scope not found'));

        $response = $this->authed()->get('/memories/nonexistent');

        $response->assertRedirect(route('memories.index'));
    }
}

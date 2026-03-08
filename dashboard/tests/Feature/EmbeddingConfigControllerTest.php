<?php

namespace Tests\Feature;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Mockery;
use Tests\TestCase;

class EmbeddingConfigControllerTest extends TestCase
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

    public function test_index_lists_configs_with_api_key_map(): void
    {
        $configs = [['id' => '1', 'name' => 'ada', 'api_key_config_id' => 'ak1']];
        $apiKeys = [['id' => 'ak1', 'name' => 'openai-prod']];
        $this->api->shouldReceive('listEmbeddingConfigs')->once()->andReturn($configs);
        $this->api->shouldReceive('listApiKeyConfigs')->once()->andReturn($apiKeys);

        $response = $this->authed()->get('/config/embedding');

        $response->assertStatus(200);
        $response->assertViewIs('config.embedding.index');
        $response->assertViewHas('configs', $configs);
        $response->assertViewHas('apiKeyMap', ['ak1' => 'openai-prod']);
    }

    public function test_index_handles_api_error(): void
    {
        $this->api->shouldReceive('listEmbeddingConfigs')
            ->andThrow(new CreelApiException(500, '', 'Error'));

        $response = $this->authed()->get('/config/embedding');

        $response->assertStatus(200);
        $response->assertSessionHas('error');
    }

    public function test_create_fetches_api_key_configs(): void
    {
        $apiKeys = [['id' => 'ak1', 'name' => 'openai']];
        $this->api->shouldReceive('listApiKeyConfigs')->once()->andReturn($apiKeys);

        $response = $this->authed()->get('/config/embedding/create');

        $response->assertStatus(200);
        $response->assertViewHas('apiKeyConfigs', $apiKeys);
    }

    public function test_store_creates_config_with_dimensions_cast_to_int(): void
    {
        $this->api->shouldReceive('createEmbeddingConfig')
            ->once()
            ->with(Mockery::on(fn ($data) =>
                $data['name'] === 'ada-small' &&
                $data['provider'] === 'openai' &&
                $data['model'] === 'text-embedding-3-small' &&
                $data['dimensions'] === 1536 &&
                $data['api_key_config_id'] === 'ak1' &&
                $data['is_default'] === true
            ))
            ->andReturn(['id' => 'new']);

        $response = $this->authed()->post('/config/embedding', [
            'name' => 'ada-small',
            'provider' => 'openai',
            'model' => 'text-embedding-3-small',
            'dimensions' => '1536',
            'api_key_config_id' => 'ak1',
            'is_default' => '1',
        ]);

        $response->assertRedirect(route('config.embedding.index'));
        $response->assertSessionHas('success');
    }

    public function test_store_validates_required_fields(): void
    {
        $response = $this->authed()->post('/config/embedding', []);

        $response->assertSessionHasErrors(['name', 'provider', 'model', 'dimensions', 'api_key_config_id']);
    }

    public function test_edit_fetches_config_and_api_keys(): void
    {
        $config = ['id' => '1', 'name' => 'ada'];
        $apiKeys = [['id' => 'ak1', 'name' => 'openai']];
        $this->api->shouldReceive('getEmbeddingConfig')->with('1')->once()->andReturn($config);
        $this->api->shouldReceive('listApiKeyConfigs')->once()->andReturn($apiKeys);

        $response = $this->authed()->get('/config/embedding/1/edit');

        $response->assertStatus(200);
        $response->assertViewHas('config', $config);
        $response->assertViewHas('apiKeyConfigs', $apiKeys);
    }

    public function test_edit_redirects_on_error(): void
    {
        $this->api->shouldReceive('getEmbeddingConfig')
            ->andThrow(new CreelApiException(404, '', 'Not found'));

        $response = $this->authed()->get('/config/embedding/bad/edit');

        $response->assertRedirect(route('config.embedding.index'));
    }

    public function test_update_only_sends_name_and_api_key_config_id(): void
    {
        $this->api->shouldReceive('updateEmbeddingConfig')
            ->once()
            ->with('1', ['name' => 'renamed', 'api_key_config_id' => 'ak2'])
            ->andReturn(['id' => '1']);

        $response = $this->authed()->patch('/config/embedding/1', [
            'name' => 'renamed',
            'api_key_config_id' => 'ak2',
        ]);

        $response->assertRedirect(route('config.embedding.index'));
    }

    public function test_destroy_and_set_default(): void
    {
        $this->api->shouldReceive('deleteEmbeddingConfig')->with('1')->once();

        $response = $this->authed()->delete('/config/embedding/1');
        $response->assertRedirect(route('config.embedding.index'));
        $response->assertSessionHas('success');

        $this->api->shouldReceive('setDefaultEmbeddingConfig')->with('2')->once()->andReturn([]);

        $response = $this->authed()->post('/config/embedding/2/default');
        $response->assertRedirect(route('config.embedding.index'));
        $response->assertSessionHas('success');
    }
}

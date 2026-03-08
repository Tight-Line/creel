<?php

namespace Tests\Feature;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Mockery;
use Tests\TestCase;

class LlmConfigControllerTest extends TestCase
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
        $configs = [['id' => '1', 'name' => 'gpt4o', 'api_key_config_id' => 'ak1']];
        $apiKeys = [['id' => 'ak1', 'name' => 'openai-prod']];
        $this->api->shouldReceive('listLlmConfigs')->once()->andReturn($configs);
        $this->api->shouldReceive('listApiKeyConfigs')->once()->andReturn($apiKeys);

        $response = $this->authed()->get('/config/llm');

        $response->assertStatus(200);
        $response->assertViewIs('config.llm.index');
        $response->assertViewHas('configs', $configs);
        $response->assertViewHas('apiKeyMap', ['ak1' => 'openai-prod']);
    }

    public function test_index_handles_api_error(): void
    {
        $this->api->shouldReceive('listLlmConfigs')
            ->andThrow(new CreelApiException(500, '', 'Error'));

        $response = $this->authed()->get('/config/llm');

        $response->assertStatus(200);
        $response->assertSessionHas('error');
        $response->assertViewHas('configs', []);
    }

    public function test_create_fetches_api_key_configs(): void
    {
        $apiKeys = [['id' => 'ak1', 'name' => 'openai']];
        $this->api->shouldReceive('listApiKeyConfigs')->once()->andReturn($apiKeys);

        $response = $this->authed()->get('/config/llm/create');

        $response->assertStatus(200);
        $response->assertViewHas('apiKeyConfigs', $apiKeys);
    }

    public function test_store_parses_parameters_and_creates(): void
    {
        $this->api->shouldReceive('createLlmConfig')
            ->once()
            ->with(Mockery::on(fn ($data) =>
                $data['name'] === 'gpt4o' &&
                $data['provider'] === 'openai' &&
                $data['model'] === 'gpt-4o' &&
                $data['api_key_config_id'] === 'ak1' &&
                $data['is_default'] === false &&
                $data['parameters'] === ['temperature' => '0.7', 'max_tokens' => '4096']
            ))
            ->andReturn(['id' => 'new']);

        $response = $this->authed()->post('/config/llm', [
            'name' => 'gpt4o',
            'provider' => 'openai',
            'model' => 'gpt-4o',
            'api_key_config_id' => 'ak1',
            'parameters' => "temperature=0.7\nmax_tokens=4096",
        ]);

        $response->assertRedirect(route('config.llm.index'));
        $response->assertSessionHas('success');
    }

    public function test_store_skips_empty_parameter_lines(): void
    {
        $this->api->shouldReceive('createLlmConfig')
            ->once()
            ->with(Mockery::on(fn ($data) =>
                $data['parameters'] === ['temperature' => '0.7']
            ))
            ->andReturn(['id' => 'new']);

        $response = $this->authed()->post('/config/llm', [
            'name' => 'test',
            'provider' => 'openai',
            'model' => 'gpt-4o',
            'api_key_config_id' => 'ak1',
            'parameters' => "\n  \ntemperature=0.7\n\n",
        ]);

        $response->assertRedirect(route('config.llm.index'));
    }

    public function test_store_omits_parameters_when_empty(): void
    {
        $this->api->shouldReceive('createLlmConfig')
            ->once()
            ->with(Mockery::on(fn ($data) => !isset($data['parameters'])))
            ->andReturn(['id' => 'new']);

        $response = $this->authed()->post('/config/llm', [
            'name' => 'test',
            'provider' => 'openai',
            'model' => 'gpt-4o',
            'api_key_config_id' => 'ak1',
        ]);

        $response->assertRedirect(route('config.llm.index'));
    }

    public function test_store_validates_required_fields(): void
    {
        $response = $this->authed()->post('/config/llm', []);

        $response->assertSessionHasErrors(['name', 'provider', 'model', 'api_key_config_id']);
    }

    public function test_edit_fetches_config_and_api_keys(): void
    {
        $config = ['id' => '1', 'name' => 'gpt4o'];
        $apiKeys = [['id' => 'ak1', 'name' => 'openai']];
        $this->api->shouldReceive('getLlmConfig')->with('1')->once()->andReturn($config);
        $this->api->shouldReceive('listApiKeyConfigs')->once()->andReturn($apiKeys);

        $response = $this->authed()->get('/config/llm/1/edit');

        $response->assertStatus(200);
        $response->assertViewHas('config', $config);
        $response->assertViewHas('apiKeyConfigs', $apiKeys);
    }

    public function test_edit_redirects_on_api_error(): void
    {
        $this->api->shouldReceive('getLlmConfig')
            ->andThrow(new CreelApiException(404, '', 'Not found'));

        $response = $this->authed()->get('/config/llm/bad/edit');

        $response->assertRedirect(route('config.llm.index'));
        $response->assertSessionHas('error');
    }

    public function test_update_parses_parameters(): void
    {
        $this->api->shouldReceive('updateLlmConfig')
            ->once()
            ->with('1', Mockery::on(fn ($data) =>
                $data['name'] === 'renamed' &&
                $data['parameters'] === ['temperature' => '0.5']
            ))
            ->andReturn(['id' => '1']);

        $response = $this->authed()->patch('/config/llm/1', [
            'name' => 'renamed',
            'provider' => '',
            'parameters' => 'temperature=0.5',
        ]);

        $response->assertRedirect(route('config.llm.index'));
    }

    public function test_destroy_deletes_and_redirects(): void
    {
        $this->api->shouldReceive('deleteLlmConfig')->with('1')->once();

        $response = $this->authed()->delete('/config/llm/1');

        $response->assertRedirect(route('config.llm.index'));
        $response->assertSessionHas('success');
    }

    public function test_set_default(): void
    {
        $this->api->shouldReceive('setDefaultLlmConfig')->with('1')->once()->andReturn([]);

        $response = $this->authed()->post('/config/llm/1/default');

        $response->assertRedirect(route('config.llm.index'));
        $response->assertSessionHas('success');
    }
}

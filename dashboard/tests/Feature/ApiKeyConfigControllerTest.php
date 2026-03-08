<?php

namespace Tests\Feature;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Mockery;
use Tests\TestCase;

class ApiKeyConfigControllerTest extends TestCase
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

    public function test_index_lists_configs(): void
    {
        $configs = [['id' => 'abc', 'name' => 'openai-prod', 'provider' => 'openai']];
        $this->api->shouldReceive('listApiKeyConfigs')->once()->andReturn($configs);

        $response = $this->authed()->get('/config/apikey');

        $response->assertStatus(200);
        $response->assertViewIs('config.apikey.index');
        $response->assertViewHas('configs', $configs);
    }

    public function test_index_handles_api_error(): void
    {
        $this->api->shouldReceive('listApiKeyConfigs')
            ->andThrow(new CreelApiException(500, '', 'Server error'));

        $response = $this->authed()->get('/config/apikey');

        $response->assertStatus(200);
        $response->assertSessionHas('error', 'Server error');
        $response->assertViewHas('configs', []);
    }

    public function test_create_returns_form(): void
    {
        $response = $this->authed()->get('/config/apikey/create');

        $response->assertStatus(200);
        $response->assertViewIs('config.apikey.create');
    }

    public function test_store_creates_config_and_redirects(): void
    {
        $this->api->shouldReceive('createApiKeyConfig')
            ->once()
            ->with(Mockery::on(fn ($data) =>
                $data['name'] === 'openai-prod' &&
                $data['provider'] === 'openai' &&
                $data['api_key'] === 'sk-test' &&
                $data['is_default'] === true
            ))
            ->andReturn(['id' => 'new-id']);

        $response = $this->authed()->post('/config/apikey', [
            'name' => 'openai-prod',
            'provider' => 'openai',
            'api_key' => 'sk-test',
            'is_default' => '1',
        ]);

        $response->assertRedirect(route('config.apikey.index'));
        $response->assertSessionHas('success');
    }

    public function test_store_validates_required_fields(): void
    {
        $response = $this->authed()->post('/config/apikey', []);

        $response->assertSessionHasErrors(['name', 'provider', 'api_key']);
    }

    public function test_store_handles_api_error(): void
    {
        $this->api->shouldReceive('createApiKeyConfig')
            ->andThrow(new CreelApiException(409, '', 'Name already exists'));

        $response = $this->authed()->post('/config/apikey', [
            'name' => 'dup',
            'provider' => 'openai',
            'api_key' => 'sk-test',
        ]);

        $response->assertRedirect();
        $response->assertSessionHas('error', 'Name already exists');
    }

    public function test_edit_returns_form_with_config(): void
    {
        $config = ['id' => 'abc', 'name' => 'openai-prod', 'provider' => 'openai'];
        $this->api->shouldReceive('getApiKeyConfig')->with('abc')->once()->andReturn($config);

        $response = $this->authed()->get('/config/apikey/abc/edit');

        $response->assertStatus(200);
        $response->assertViewIs('config.apikey.edit');
        $response->assertViewHas('config', $config);
    }

    public function test_edit_redirects_on_api_error(): void
    {
        $this->api->shouldReceive('getApiKeyConfig')
            ->andThrow(new CreelApiException(404, '', 'Not found'));

        $response = $this->authed()->get('/config/apikey/bad-id/edit');

        $response->assertRedirect(route('config.apikey.index'));
        $response->assertSessionHas('error', 'Not found');
    }

    public function test_update_sends_only_filled_fields(): void
    {
        $this->api->shouldReceive('updateApiKeyConfig')
            ->once()
            ->with('abc', ['name' => 'renamed'])
            ->andReturn(['id' => 'abc']);

        $response = $this->authed()->patch('/config/apikey/abc', [
            'name' => 'renamed',
            'provider' => '',
            'api_key' => '',
        ]);

        $response->assertRedirect(route('config.apikey.index'));
        $response->assertSessionHas('success');
    }

    public function test_update_handles_api_error(): void
    {
        $this->api->shouldReceive('updateApiKeyConfig')
            ->andThrow(new CreelApiException(400, '', 'Bad request'));

        $response = $this->authed()->patch('/config/apikey/abc', ['name' => 'x']);

        $response->assertRedirect();
        $response->assertSessionHas('error', 'Bad request');
    }

    public function test_destroy_deletes_and_redirects(): void
    {
        $this->api->shouldReceive('deleteApiKeyConfig')->with('abc')->once();

        $response = $this->authed()->delete('/config/apikey/abc');

        $response->assertRedirect(route('config.apikey.index'));
        $response->assertSessionHas('success');
    }

    public function test_destroy_handles_api_error(): void
    {
        $this->api->shouldReceive('deleteApiKeyConfig')
            ->andThrow(new CreelApiException(409, '', 'In use'));

        $response = $this->authed()->delete('/config/apikey/abc');

        $response->assertRedirect(route('config.apikey.index'));
        $response->assertSessionHas('error', 'In use');
    }

    public function test_set_default_and_redirects(): void
    {
        $this->api->shouldReceive('setDefaultApiKeyConfig')->with('abc')->once()->andReturn([]);

        $response = $this->authed()->post('/config/apikey/abc/default');

        $response->assertRedirect(route('config.apikey.index'));
        $response->assertSessionHas('success');
    }
}

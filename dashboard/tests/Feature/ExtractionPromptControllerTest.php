<?php

namespace Tests\Feature;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Mockery;
use Tests\TestCase;

class ExtractionPromptControllerTest extends TestCase
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
        $configs = [['id' => '1', 'name' => 'default', 'prompt' => 'Extract...']];
        $this->api->shouldReceive('listPromptConfigs')->once()->andReturn($configs);

        $response = $this->authed()->get('/config/prompt');

        $response->assertStatus(200);
        $response->assertViewIs('config.prompt.index');
        $response->assertViewHas('configs', $configs);
    }

    public function test_index_handles_api_error(): void
    {
        $this->api->shouldReceive('listPromptConfigs')
            ->andThrow(new CreelApiException(500, '', 'Error'));

        $response = $this->authed()->get('/config/prompt');

        $response->assertStatus(200);
        $response->assertSessionHas('error');
        $response->assertViewHas('configs', []);
    }

    public function test_create_returns_form(): void
    {
        $response = $this->authed()->get('/config/prompt/create');

        $response->assertStatus(200);
        $response->assertViewIs('config.prompt.create');
    }

    public function test_store_creates_with_description(): void
    {
        $this->api->shouldReceive('createPromptConfig')
            ->once()
            ->with(Mockery::on(fn ($data) =>
                $data['name'] === 'extraction-v1' &&
                $data['prompt'] === 'Extract key facts...' &&
                $data['description'] === 'Standard extraction' &&
                $data['is_default'] === true
            ))
            ->andReturn(['id' => 'new']);

        $response = $this->authed()->post('/config/prompt', [
            'name' => 'extraction-v1',
            'prompt' => 'Extract key facts...',
            'description' => 'Standard extraction',
            'is_default' => '1',
        ]);

        $response->assertRedirect(route('config.prompt.index'));
        $response->assertSessionHas('success');
    }

    public function test_store_omits_empty_description(): void
    {
        $this->api->shouldReceive('createPromptConfig')
            ->once()
            ->with(Mockery::on(fn ($data) => !isset($data['description'])))
            ->andReturn(['id' => 'new']);

        $response = $this->authed()->post('/config/prompt', [
            'name' => 'test',
            'prompt' => 'Do something',
        ]);

        $response->assertRedirect(route('config.prompt.index'));
    }

    public function test_store_validates_required_fields(): void
    {
        $response = $this->authed()->post('/config/prompt', []);

        $response->assertSessionHasErrors(['name', 'prompt']);
    }

    public function test_edit_fetches_config(): void
    {
        $config = ['id' => '1', 'name' => 'test', 'prompt' => 'Extract...'];
        $this->api->shouldReceive('getPromptConfig')->with('1')->once()->andReturn($config);

        $response = $this->authed()->get('/config/prompt/1/edit');

        $response->assertStatus(200);
        $response->assertViewHas('config', $config);
    }

    public function test_edit_redirects_on_error(): void
    {
        $this->api->shouldReceive('getPromptConfig')
            ->andThrow(new CreelApiException(404, '', 'Not found'));

        $response = $this->authed()->get('/config/prompt/bad/edit');

        $response->assertRedirect(route('config.prompt.index'));
    }

    public function test_update_filters_empty_values(): void
    {
        $this->api->shouldReceive('updatePromptConfig')
            ->once()
            ->with('1', ['prompt' => 'New prompt text'])
            ->andReturn(['id' => '1']);

        $response = $this->authed()->patch('/config/prompt/1', [
            'name' => '',
            'prompt' => 'New prompt text',
            'description' => '',
        ]);

        $response->assertRedirect(route('config.prompt.index'));
    }

    public function test_destroy(): void
    {
        $this->api->shouldReceive('deletePromptConfig')->with('1')->once();

        $response = $this->authed()->delete('/config/prompt/1');

        $response->assertRedirect(route('config.prompt.index'));
        $response->assertSessionHas('success');
    }

    public function test_set_default(): void
    {
        $this->api->shouldReceive('setDefaultPromptConfig')->with('1')->once()->andReturn([]);

        $response = $this->authed()->post('/config/prompt/1/default');

        $response->assertRedirect(route('config.prompt.index'));
        $response->assertSessionHas('success');
    }
}

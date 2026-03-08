<?php

namespace Tests\Feature;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Mockery;
use Tests\TestCase;

class TopicControllerTest extends TestCase
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

    public function test_index_lists_topics(): void
    {
        $topics = [['id' => '1', 'name' => 'my-topic']];
        $this->api->shouldReceive('listTopics')->once()->andReturn($topics);

        $response = $this->authed()->get('/topics');

        $response->assertStatus(200);
        $response->assertViewIs('topics.index');
        $response->assertViewHas('topics', $topics);
    }

    public function test_index_handles_api_error(): void
    {
        $this->api->shouldReceive('listTopics')
            ->andThrow(new CreelApiException(500, '', 'Error'));

        $response = $this->authed()->get('/topics');

        $response->assertStatus(200);
        $response->assertSessionHas('error');
        $response->assertViewHas('topics', []);
    }

    public function test_edit_fetches_topic_and_all_config_lists(): void
    {
        $topic = ['id' => '1', 'name' => 'my-topic', 'description' => 'A test topic'];
        $llm = [['id' => 'l1', 'name' => 'gpt4o']];
        $emb = [['id' => 'e1', 'name' => 'ada']];
        $prompts = [['id' => 'p1', 'name' => 'default']];
        $this->api->shouldReceive('getTopic')->with('1')->once()->andReturn($topic);
        $this->api->shouldReceive('listLlmConfigs')->once()->andReturn($llm);
        $this->api->shouldReceive('listEmbeddingConfigs')->once()->andReturn($emb);
        $this->api->shouldReceive('listPromptConfigs')->once()->andReturn($prompts);

        $response = $this->authed()->get('/topics/1/edit');

        $response->assertStatus(200);
        $response->assertViewIs('topics.edit');
        $response->assertViewHas('topic', $topic);
        $response->assertViewHas('llmConfigs', $llm);
        $response->assertViewHas('embeddingConfigs', $emb);
        $response->assertViewHas('promptConfigs', $prompts);
    }

    public function test_edit_redirects_on_api_error(): void
    {
        $this->api->shouldReceive('getTopic')
            ->andThrow(new CreelApiException(404, '', 'Not found'));

        $response = $this->authed()->get('/topics/bad/edit');

        $response->assertRedirect(route('topics.index'));
    }

    public function test_update_sends_filled_fields(): void
    {
        $this->api->shouldReceive('updateTopic')
            ->once()
            ->with('1', ['name' => 'renamed', 'llm_config_id' => 'l1'])
            ->andReturn(['id' => '1']);

        $response = $this->authed()->patch('/topics/1', [
            'name' => 'renamed',
            'description' => '',
            'llm_config_id' => 'l1',
            'embedding_config_id' => '',
            'extraction_prompt_config_id' => '',
        ]);

        $response->assertRedirect(route('topics.index'));
        $response->assertSessionHas('success');
    }

    public function test_update_handles_api_error(): void
    {
        $this->api->shouldReceive('updateTopic')
            ->andThrow(new CreelApiException(400, '', 'Extraction prompt requires LLM config'));

        $response = $this->authed()->patch('/topics/1', [
            'extraction_prompt_config_id' => 'p1',
        ]);

        $response->assertRedirect();
        $response->assertSessionHas('error', 'Extraction prompt requires LLM config');
    }
}

<?php

namespace Tests\Unit;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Illuminate\Http\Client\ConnectionException;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class CreelApiClientTest extends TestCase
{
    private CreelApiClient $client;

    protected function setUp(): void
    {
        parent::setUp();

        config(['creel.endpoint' => 'http://creel-test:8080']);
        config(['creel.api_key' => 'test-api-key']);

        $this->client = new CreelApiClient();
    }

    // ---------------------------------------------------------------
    // request() internals
    // ---------------------------------------------------------------

    public function test_get_request_sends_correct_method_url_and_bearer_token(): void
    {
        Http::fake([
            'creel-test:8080/v1/health' => Http::response(['status' => 'ok']),
        ]);

        $this->client->health();

        Http::assertSent(function ($request) {
            return $request->method() === 'GET'
                && $request->url() === 'http://creel-test:8080/v1/health'
                && $request->hasHeader('Authorization', 'Bearer test-api-key');
        });
    }

    public function test_post_request_sends_correct_method_and_body(): void
    {
        Http::fake([
            'creel-test:8080/v1/config/apikey' => Http::response(['id' => 'ak-1', 'name' => 'new']),
        ]);

        $data = ['name' => 'new', 'permissions' => ['read']];
        $this->client->createApiKeyConfig($data);

        Http::assertSent(function ($request) use ($data) {
            return $request->method() === 'POST'
                && $request->url() === 'http://creel-test:8080/v1/config/apikey'
                && $request->data() === $data;
        });
    }

    public function test_patch_request_sends_correct_method_and_url(): void
    {
        Http::fake([
            'creel-test:8080/v1/config/llm/llm-42' => Http::response(['id' => 'llm-42']),
        ]);

        $this->client->updateLlmConfig('llm-42', ['model' => 'gpt-4']);

        Http::assertSent(function ($request) {
            return $request->method() === 'PATCH'
                && $request->url() === 'http://creel-test:8080/v1/config/llm/llm-42';
        });
    }

    public function test_delete_request_sends_correct_method_and_url(): void
    {
        Http::fake([
            'creel-test:8080/v1/config/embedding/emb-7' => Http::response([], 200),
        ]);

        $this->client->deleteEmbeddingConfig('emb-7');

        Http::assertSent(function ($request) {
            return $request->method() === 'DELETE'
                && $request->url() === 'http://creel-test:8080/v1/config/embedding/emb-7';
        });
    }

    public function test_connection_exception_is_wrapped_as_creel_api_exception(): void
    {
        Http::fake(fn () => throw new ConnectionException('Connection refused'));

        $this->expectException(CreelApiException::class);
        $this->expectExceptionMessage('Cannot connect to Creel API at http://creel-test:8080: Connection refused');

        $this->client->health();
    }

    public function test_http_error_with_json_message_throws_creel_api_exception(): void
    {
        Http::fake([
            'creel-test:8080/v1/health' => Http::response(
                ['message' => 'Unauthorized: invalid token'],
                401,
            ),
        ]);

        try {
            $this->client->health();
            $this->fail('Expected CreelApiException was not thrown');
        } catch (CreelApiException $e) {
            $this->assertSame(401, $e->statusCode);
            $this->assertSame('Unauthorized: invalid token', $e->getMessage());
        }
    }

    public function test_http_error_with_non_json_body_throws_creel_api_exception_with_raw_body(): void
    {
        Http::fake([
            'creel-test:8080/v1/health' => Http::response('Internal Server Error', 500),
        ]);

        try {
            $this->client->health();
            $this->fail('Expected CreelApiException was not thrown');
        } catch (CreelApiException $e) {
            $this->assertSame(500, $e->statusCode);
            $this->assertSame('Internal Server Error', $e->getMessage());
            $this->assertSame('Internal Server Error', $e->errorBody);
        }
    }

    public function test_successful_response_returns_decoded_json(): void
    {
        Http::fake([
            'creel-test:8080/v1/health' => Http::response(['status' => 'ok', 'version' => '1.2.3']),
        ]);

        $result = $this->client->health();

        $this->assertSame(['status' => 'ok', 'version' => '1.2.3'], $result);
    }

    public function test_successful_response_with_null_json_returns_empty_array(): void
    {
        Http::fake([
            'creel-test:8080/v1/health' => Http::response('', 200),
        ]);

        $result = $this->client->health();

        $this->assertSame([], $result);
    }

    // ---------------------------------------------------------------
    // Public method routing
    // ---------------------------------------------------------------

    public function test_health_calls_get_v1_health(): void
    {
        Http::fake([
            'creel-test:8080/v1/health' => Http::response(['status' => 'ok']),
        ]);

        $result = $this->client->health();

        $this->assertSame(['status' => 'ok'], $result);
        Http::assertSent(fn ($r) => $r->method() === 'GET' && str_ends_with($r->url(), '/v1/health'));
    }

    public function test_list_api_key_configs_unwraps_configs_key(): void
    {
        $configs = [['id' => 'ak-1'], ['id' => 'ak-2']];
        Http::fake([
            'creel-test:8080/v1/config/apikey' => Http::response(['configs' => $configs]),
        ]);

        $result = $this->client->listApiKeyConfigs();

        $this->assertSame($configs, $result);
    }

    public function test_list_api_key_configs_returns_empty_array_when_key_missing(): void
    {
        Http::fake([
            'creel-test:8080/v1/config/apikey' => Http::response([]),
        ]);

        $result = $this->client->listApiKeyConfigs();

        $this->assertSame([], $result);
    }

    public function test_list_llm_configs_unwraps_configs_key(): void
    {
        $configs = [['id' => 'llm-1', 'model' => 'gpt-4']];
        Http::fake([
            'creel-test:8080/v1/config/llm' => Http::response(['configs' => $configs]),
        ]);

        $result = $this->client->listLlmConfigs();

        $this->assertSame($configs, $result);
    }

    public function test_list_topics_unwraps_topics_key(): void
    {
        $topics = [['id' => 't-1', 'name' => 'fishing'], ['id' => 't-2', 'name' => 'skiing']];
        Http::fake([
            'creel-test:8080/v1/topics' => Http::response(['topics' => $topics]),
        ]);

        $result = $this->client->listTopics();

        $this->assertSame($topics, $result);
    }

    public function test_list_system_accounts_unwraps_accounts_key(): void
    {
        $accounts = [['id' => 'sa-1', 'name' => 'system']];
        Http::fake([
            'creel-test:8080/v1/admin/accounts' => Http::response(['accounts' => $accounts]),
        ]);

        $result = $this->client->listSystemAccounts();

        $this->assertSame($accounts, $result);
    }

    public function test_create_api_key_config_sends_post_with_data(): void
    {
        $payload = ['name' => 'my-key', 'permissions' => ['admin']];
        $response = ['id' => 'ak-new', 'name' => 'my-key'];

        Http::fake([
            'creel-test:8080/v1/config/apikey' => Http::response($response),
        ]);

        $result = $this->client->createApiKeyConfig($payload);

        $this->assertSame($response, $result);
        Http::assertSent(function ($request) use ($payload) {
            return $request->method() === 'POST'
                && $request->url() === 'http://creel-test:8080/v1/config/apikey'
                && $request->data() === $payload;
        });
    }

    public function test_update_llm_config_sends_patch_with_id_and_data(): void
    {
        $payload = ['model' => 'claude-3'];
        $response = ['id' => 'llm-5', 'model' => 'claude-3'];

        Http::fake([
            'creel-test:8080/v1/config/llm/llm-5' => Http::response($response),
        ]);

        $result = $this->client->updateLlmConfig('llm-5', $payload);

        $this->assertSame($response, $result);
        Http::assertSent(function ($request) use ($payload) {
            return $request->method() === 'PATCH'
                && $request->url() === 'http://creel-test:8080/v1/config/llm/llm-5'
                && $request->data() === $payload;
        });
    }

    public function test_delete_embedding_config_sends_delete_with_id(): void
    {
        Http::fake([
            'creel-test:8080/v1/config/embedding/emb-9' => Http::response([], 200),
        ]);

        $this->client->deleteEmbeddingConfig('emb-9');

        Http::assertSent(function ($request) {
            return $request->method() === 'DELETE'
                && $request->url() === 'http://creel-test:8080/v1/config/embedding/emb-9';
        });
    }

    public function test_set_default_prompt_config_sends_post_to_default_endpoint(): void
    {
        Http::fake([
            'creel-test:8080/v1/config/prompt/pr-3/default' => Http::response(['id' => 'pr-3', 'is_default' => true]),
        ]);

        $result = $this->client->setDefaultPromptConfig('pr-3');

        $this->assertSame(['id' => 'pr-3', 'is_default' => true], $result);
        Http::assertSent(function ($request) {
            return $request->method() === 'POST'
                && $request->url() === 'http://creel-test:8080/v1/config/prompt/pr-3/default';
        });
    }

    public function test_rotate_key_sends_grace_period_seconds_in_body(): void
    {
        Http::fake([
            'creel-test:8080/v1/admin/accounts/sa-1/rotate' => Http::response(['new_key' => 'sk-rotated']),
        ]);

        $result = $this->client->rotateKey('sa-1', 3600);

        $this->assertSame(['new_key' => 'sk-rotated'], $result);
        Http::assertSent(function ($request) {
            return $request->method() === 'POST'
                && $request->url() === 'http://creel-test:8080/v1/admin/accounts/sa-1/rotate'
                && $request->data() === ['grace_period_seconds' => 3600];
        });
    }

    public function test_rotate_key_defaults_grace_period_to_zero(): void
    {
        Http::fake([
            'creel-test:8080/v1/admin/accounts/sa-2/rotate' => Http::response(['new_key' => 'sk-new']),
        ]);

        $this->client->rotateKey('sa-2');

        Http::assertSent(function ($request) {
            return $request->data() === ['grace_period_seconds' => 0];
        });
    }
}

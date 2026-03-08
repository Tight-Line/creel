<?php

namespace App\Services;

use App\Exceptions\CreelApiException;
use Illuminate\Support\Facades\Http;

class CreelApiClient
{
    private string $baseUrl;
    private string $apiKey;

    public function __construct()
    {
        $this->baseUrl = rtrim(config('creel.endpoint'), '/');
        $this->apiKey = config('creel.api_key');
    }

    // Health

    public function health(): array
    {
        return $this->request('GET', '/v1/health');
    }

    // API Key Configs

    public function listApiKeyConfigs(): array
    {
        return $this->request('GET', '/v1/config/apikey')['configs'] ?? [];
    }

    public function getApiKeyConfig(string $id): array
    {
        return $this->request('GET', "/v1/config/apikey/{$id}");
    }

    public function createApiKeyConfig(array $data): array
    {
        return $this->request('POST', '/v1/config/apikey', $data);
    }

    public function updateApiKeyConfig(string $id, array $data): array
    {
        return $this->request('PATCH', "/v1/config/apikey/{$id}", $data);
    }

    public function deleteApiKeyConfig(string $id): void
    {
        $this->request('DELETE', "/v1/config/apikey/{$id}");
    }

    public function setDefaultApiKeyConfig(string $id): array
    {
        return $this->request('POST', "/v1/config/apikey/{$id}/default");
    }

    // LLM Configs

    public function listLlmConfigs(): array
    {
        return $this->request('GET', '/v1/config/llm')['configs'] ?? [];
    }

    public function getLlmConfig(string $id): array
    {
        return $this->request('GET', "/v1/config/llm/{$id}");
    }

    public function createLlmConfig(array $data): array
    {
        return $this->request('POST', '/v1/config/llm', $data);
    }

    public function updateLlmConfig(string $id, array $data): array
    {
        return $this->request('PATCH', "/v1/config/llm/{$id}", $data);
    }

    public function deleteLlmConfig(string $id): void
    {
        $this->request('DELETE', "/v1/config/llm/{$id}");
    }

    public function setDefaultLlmConfig(string $id): array
    {
        return $this->request('POST', "/v1/config/llm/{$id}/default");
    }

    // Embedding Configs

    public function listEmbeddingConfigs(): array
    {
        return $this->request('GET', '/v1/config/embedding')['configs'] ?? [];
    }

    public function getEmbeddingConfig(string $id): array
    {
        return $this->request('GET', "/v1/config/embedding/{$id}");
    }

    public function createEmbeddingConfig(array $data): array
    {
        return $this->request('POST', '/v1/config/embedding', $data);
    }

    public function updateEmbeddingConfig(string $id, array $data): array
    {
        return $this->request('PATCH', "/v1/config/embedding/{$id}", $data);
    }

    public function deleteEmbeddingConfig(string $id): void
    {
        $this->request('DELETE', "/v1/config/embedding/{$id}");
    }

    public function setDefaultEmbeddingConfig(string $id): array
    {
        return $this->request('POST', "/v1/config/embedding/{$id}/default");
    }

    // Extraction Prompt Configs

    public function listPromptConfigs(): array
    {
        return $this->request('GET', '/v1/config/prompt')['configs'] ?? [];
    }

    public function getPromptConfig(string $id): array
    {
        return $this->request('GET', "/v1/config/prompt/{$id}");
    }

    public function createPromptConfig(array $data): array
    {
        return $this->request('POST', '/v1/config/prompt', $data);
    }

    public function updatePromptConfig(string $id, array $data): array
    {
        return $this->request('PATCH', "/v1/config/prompt/{$id}", $data);
    }

    public function deletePromptConfig(string $id): void
    {
        $this->request('DELETE', "/v1/config/prompt/{$id}");
    }

    public function setDefaultPromptConfig(string $id): array
    {
        return $this->request('POST', "/v1/config/prompt/{$id}/default");
    }

    // Topics

    public function listTopics(): array
    {
        return $this->request('GET', '/v1/topics')['topics'] ?? [];
    }

    public function getTopic(string $id): array
    {
        return $this->request('GET', "/v1/topics/{$id}");
    }

    public function updateTopic(string $id, array $data): array
    {
        return $this->request('PATCH', "/v1/topics/{$id}", $data);
    }

    // System Accounts

    public function listSystemAccounts(): array
    {
        return $this->request('GET', '/v1/admin/accounts')['accounts'] ?? [];
    }

    public function createSystemAccount(array $data): array
    {
        return $this->request('POST', '/v1/admin/accounts', $data);
    }

    public function deleteSystemAccount(string $id): void
    {
        $this->request('DELETE', "/v1/admin/accounts/{$id}");
    }

    public function rotateKey(string $accountId, int $gracePeriod = 0): array
    {
        return $this->request('POST', "/v1/admin/accounts/{$accountId}/rotate", [
            'grace_period_seconds' => $gracePeriod,
        ]);
    }

    public function revokeKey(string $accountId): void
    {
        $this->request('POST', "/v1/admin/accounts/{$accountId}/revoke");
    }

    // Internal

    private function request(string $method, string $path, array $data = []): array
    {
        $url = $this->baseUrl . $path;

        $pending = Http::withToken($this->apiKey)
            ->acceptJson()
            ->timeout(10);

        $response = match ($method) {
            'GET' => $pending->get($url),
            'POST' => $pending->post($url, $data),
            'PATCH' => $pending->patch($url, $data),
            'DELETE' => $pending->delete($url),
        };

        if ($response->failed()) {
            $body = $response->body();
            $message = '';
            $decoded = json_decode($body, true);
            if (is_array($decoded) && isset($decoded['message'])) {
                $message = $decoded['message'];
            } else {
                $message = $body;
            }
            throw new CreelApiException($response->status(), $body, $message);
        }

        return $response->json() ?? [];
    }
}

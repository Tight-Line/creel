<?php

namespace App\Http\Controllers;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;

class DashboardController extends Controller
{
    public function __construct(
        private CreelApiClient $api,
    ) {}

    public function index()
    {
        $health = null;
        $counts = [
            'api_key_configs' => 0,
            'llm_configs' => 0,
            'embedding_configs' => 0,
            'prompt_configs' => 0,
            'topics' => 0,
            'system_accounts' => 0,
        ];

        try {
            $health = $this->api->health();
            $counts['api_key_configs'] = count($this->api->listApiKeyConfigs());
            $counts['llm_configs'] = count($this->api->listLlmConfigs());
            $counts['embedding_configs'] = count($this->api->listEmbeddingConfigs());
            $counts['prompt_configs'] = count($this->api->listPromptConfigs());
            $counts['topics'] = count($this->api->listTopics());
            $counts['system_accounts'] = count($this->api->listSystemAccounts());
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
        }

        return view('dashboard.index', [
            'health' => $health,
            'counts' => $counts,
        ]);
    }
}

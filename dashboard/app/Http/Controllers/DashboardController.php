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
            'extraction_prompt_configs' => 0,
            'topics' => 0,
            'system_accounts' => 0,
            'documents' => 0,
            'chunks' => 0,
            'memories' => 0,
        ];

        try {
            $health = $this->api->health();
            $stats = $this->api->getStats();
            $counts = array_merge($counts, array_intersect_key($stats, $counts));
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
        }

        return view('dashboard.index', [
            'health' => $health,
            'counts' => $counts,
        ]);
    }
}

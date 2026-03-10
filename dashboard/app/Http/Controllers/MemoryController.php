<?php

namespace App\Http\Controllers;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;

class MemoryController extends Controller
{
    public function __construct(
        private CreelApiClient $api,
    ) {}

    public function index()
    {
        try {
            $scopes = $this->api->listMemoryScopes();
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            $scopes = [];
        }

        return view('memories.index', ['scopes' => $scopes]);
    }

    public function scope(string $scope)
    {
        try {
            $memories = $this->api->listMemories($scope);
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->route('memories.index');
        }

        return view('memories.scope', [
            'scope' => $scope,
            'memories' => $memories,
        ]);
    }
}

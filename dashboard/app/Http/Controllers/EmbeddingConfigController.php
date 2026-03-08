<?php

namespace App\Http\Controllers;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Illuminate\Http\Request;

class EmbeddingConfigController extends Controller
{
    public function __construct(
        private CreelApiClient $api,
    ) {}

    public function index()
    {
        try {
            $configs = $this->api->listEmbeddingConfigs();
            $apiKeyConfigs = $this->api->listApiKeyConfigs();
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            $configs = [];
            $apiKeyConfigs = [];
        }

        $apiKeyMap = collect($apiKeyConfigs)->pluck('name', 'id')->all();

        return view('config.embedding.index', [
            'configs' => $configs,
            'apiKeyMap' => $apiKeyMap,
        ]);
    }

    public function create()
    {
        try {
            $apiKeyConfigs = $this->api->listApiKeyConfigs();
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            $apiKeyConfigs = [];
        }

        return view('config.embedding.create', ['apiKeyConfigs' => $apiKeyConfigs]);
    }

    public function store(Request $request)
    {
        $request->validate([
            'name' => 'required|string',
            'provider' => 'required|string',
            'model' => 'required|string',
            'dimensions' => 'required|integer',
            'api_key_config_id' => 'required|string',
        ]);

        $data = $request->only(['name', 'provider', 'model', 'api_key_config_id']);
        $data['dimensions'] = (int) $request->input('dimensions');
        $data['is_default'] = $request->boolean('is_default');

        try {
            $this->api->createEmbeddingConfig($data);
            session()->flash('success', 'Embedding config created.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->back()->withInput();
        }

        return redirect()->route('config.embedding.index');
    }

    public function edit(string $id)
    {
        try {
            $config = $this->api->getEmbeddingConfig($id);
            $apiKeyConfigs = $this->api->listApiKeyConfigs();
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->route('config.embedding.index');
        }

        return view('config.embedding.edit', [
            'config' => $config,
            'apiKeyConfigs' => $apiKeyConfigs,
        ]);
    }

    public function update(Request $request, string $id)
    {
        $payload = array_filter(
            $request->only(['name', 'api_key_config_id']),
            fn ($v) => filled($v),
        );

        try {
            $this->api->updateEmbeddingConfig($id, $payload);
            session()->flash('success', 'Embedding config updated.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->back()->withInput();
        }

        return redirect()->route('config.embedding.index');
    }

    public function destroy(string $id)
    {
        try {
            $this->api->deleteEmbeddingConfig($id);
            session()->flash('success', 'Embedding config deleted.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
        }

        return redirect()->route('config.embedding.index');
    }

    public function setDefault(string $id)
    {
        try {
            $this->api->setDefaultEmbeddingConfig($id);
            session()->flash('success', 'Default embedding config updated.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
        }

        return redirect()->route('config.embedding.index');
    }
}

<?php

namespace App\Http\Controllers;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Illuminate\Http\Request;

class LlmConfigController extends Controller
{
    public function __construct(
        private CreelApiClient $api,
    ) {}

    public function index()
    {
        try {
            $configs = $this->api->listLlmConfigs();
            $apiKeyConfigs = $this->api->listApiKeyConfigs();
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            $configs = [];
            $apiKeyConfigs = [];
        }

        $apiKeyMap = collect($apiKeyConfigs)->pluck('name', 'id')->all();

        return view('config.llm.index', [
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

        return view('config.llm.create', ['apiKeyConfigs' => $apiKeyConfigs]);
    }

    public function store(Request $request)
    {
        $request->validate([
            'name' => 'required|string',
            'provider' => 'required|string',
            'model' => 'required|string',
            'api_key_config_id' => 'required|string',
        ]);

        $data = $request->only(['name', 'provider', 'model', 'api_key_config_id']);
        $data['is_default'] = $request->boolean('is_default');

        if ($request->filled('parameters')) {
            $params = [];
            foreach (explode("\n", $request->input('parameters')) as $line) {
                $line = trim($line);
                if ($line === '') {
                    continue;
                }
                [$key, $value] = array_pad(explode('=', $line, 2), 2, '');
                $params[trim($key)] = trim($value);
            }
            if (!empty($params)) {
                $data['parameters'] = $params;
            }
        }

        try {
            $this->api->createLlmConfig($data);
            session()->flash('success', 'LLM config created.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->back()->withInput();
        }

        return redirect()->route('config.llm.index');
    }

    public function edit(string $id)
    {
        try {
            $config = $this->api->getLlmConfig($id);
            $apiKeyConfigs = $this->api->listApiKeyConfigs();
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->route('config.llm.index');
        }

        return view('config.llm.edit', [
            'config' => $config,
            'apiKeyConfigs' => $apiKeyConfigs,
        ]);
    }

    public function update(Request $request, string $id)
    {
        $payload = array_filter(
            $request->only(['name', 'provider', 'model', 'api_key_config_id']),
            fn ($v) => filled($v),
        );

        if ($request->filled('parameters')) {
            $params = [];
            foreach (explode("\n", $request->input('parameters')) as $line) {
                $line = trim($line);
                if ($line === '') {
                    continue;
                }
                [$key, $value] = array_pad(explode('=', $line, 2), 2, '');
                $params[trim($key)] = trim($value);
            }
            if (!empty($params)) {
                $payload['parameters'] = $params;
            }
        }

        try {
            $this->api->updateLlmConfig($id, $payload);
            session()->flash('success', 'LLM config updated.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->back()->withInput();
        }

        return redirect()->route('config.llm.index');
    }

    public function destroy(string $id)
    {
        try {
            $this->api->deleteLlmConfig($id);
            session()->flash('success', 'LLM config deleted.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
        }

        return redirect()->route('config.llm.index');
    }

    public function setDefault(string $id)
    {
        try {
            $this->api->setDefaultLlmConfig($id);
            session()->flash('success', 'Default LLM config updated.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
        }

        return redirect()->route('config.llm.index');
    }
}

<?php

namespace App\Http\Controllers;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Illuminate\Http\Request;

class ApiKeyConfigController extends Controller
{
    public function __construct(
        private CreelApiClient $api,
    ) {}

    public function index()
    {
        try {
            $configs = $this->api->listApiKeyConfigs();
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            $configs = [];
        }

        return view('config.apikey.index', ['configs' => $configs]);
    }

    public function create()
    {
        return view('config.apikey.create');
    }

    public function store(Request $request)
    {
        $request->validate([
            'name' => 'required|string',
            'provider' => 'required|string',
            'api_key' => 'required|string',
        ]);

        try {
            $data = $request->only(['name', 'provider', 'api_key']);
            $data['is_default'] = $request->boolean('is_default');
            $this->api->createApiKeyConfig($data);
            session()->flash('success', 'API key config created.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->back()->withInput();
        }

        return redirect()->route('config.apikey.index');
    }

    public function edit(string $id)
    {
        try {
            $config = $this->api->getApiKeyConfig($id);
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->route('config.apikey.index');
        }

        return view('config.apikey.edit', ['config' => $config]);
    }

    public function update(Request $request, string $id)
    {
        $payload = array_filter($request->only(['name', 'provider', 'api_key']), fn ($v) => filled($v));

        try {
            $this->api->updateApiKeyConfig($id, $payload);
            session()->flash('success', 'API key config updated.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->back()->withInput();
        }

        return redirect()->route('config.apikey.index');
    }

    public function destroy(string $id)
    {
        try {
            $this->api->deleteApiKeyConfig($id);
            session()->flash('success', 'API key config deleted.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
        }

        return redirect()->route('config.apikey.index');
    }

    public function setDefault(string $id)
    {
        try {
            $this->api->setDefaultApiKeyConfig($id);
            session()->flash('success', 'Default API key config updated.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
        }

        return redirect()->route('config.apikey.index');
    }
}

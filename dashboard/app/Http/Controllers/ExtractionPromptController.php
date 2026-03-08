<?php

namespace App\Http\Controllers;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Illuminate\Http\Request;

class ExtractionPromptController extends Controller
{
    public function __construct(
        private CreelApiClient $api,
    ) {}

    public function index()
    {
        try {
            $configs = $this->api->listPromptConfigs();
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            $configs = [];
        }

        return view('config.prompt.index', ['configs' => $configs]);
    }

    public function create()
    {
        return view('config.prompt.create');
    }

    public function store(Request $request)
    {
        $request->validate([
            'name' => 'required|string',
            'prompt' => 'required|string',
            'description' => 'nullable|string',
        ]);

        $data = $request->only(['name', 'prompt']);
        $data['is_default'] = $request->boolean('is_default');
        if ($request->filled('description')) {
            $data['description'] = $request->input('description');
        }

        try {
            $this->api->createPromptConfig($data);
            session()->flash('success', 'Extraction prompt config created.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->back()->withInput();
        }

        return redirect()->route('config.prompt.index');
    }

    public function edit(string $id)
    {
        try {
            $config = $this->api->getPromptConfig($id);
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->route('config.prompt.index');
        }

        return view('config.prompt.edit', ['config' => $config]);
    }

    public function update(Request $request, string $id)
    {
        $payload = array_filter(
            $request->only(['name', 'prompt', 'description']),
            fn ($v) => filled($v),
        );

        try {
            $this->api->updatePromptConfig($id, $payload);
            session()->flash('success', 'Extraction prompt config updated.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->back()->withInput();
        }

        return redirect()->route('config.prompt.index');
    }

    public function destroy(string $id)
    {
        try {
            $this->api->deletePromptConfig($id);
            session()->flash('success', 'Extraction prompt config deleted.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
        }

        return redirect()->route('config.prompt.index');
    }

    public function setDefault(string $id)
    {
        try {
            $this->api->setDefaultPromptConfig($id);
            session()->flash('success', 'Default extraction prompt config updated.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
        }

        return redirect()->route('config.prompt.index');
    }
}

<?php

namespace App\Http\Controllers;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Illuminate\Http\Request;

class TopicController extends Controller
{
    public function __construct(
        private CreelApiClient $api,
    ) {}

    public function index()
    {
        try {
            $topics = $this->api->listTopics();
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            $topics = [];
        }

        return view('topics.index', ['topics' => $topics]);
    }

    public function edit(string $id)
    {
        try {
            $topic = $this->api->getTopic($id);
            $llmConfigs = $this->api->listLlmConfigs();
            $embeddingConfigs = $this->api->listEmbeddingConfigs();
            $promptConfigs = $this->api->listPromptConfigs();
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->route('topics.index');
        }

        return view('topics.edit', [
            'topic' => $topic,
            'llmConfigs' => $llmConfigs,
            'embeddingConfigs' => $embeddingConfigs,
            'promptConfigs' => $promptConfigs,
        ]);
    }

    public function update(Request $request, string $id)
    {
        $payload = array_filter(
            $request->only([
                'name',
                'description',
                'llm_config_id',
                'embedding_config_id',
                'extraction_prompt_config_id',
            ]),
            fn ($v) => filled($v),
        );

        try {
            $this->api->updateTopic($id, $payload);
            session()->flash('success', 'Topic updated.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->back()->withInput();
        }

        return redirect()->route('topics.index');
    }
}

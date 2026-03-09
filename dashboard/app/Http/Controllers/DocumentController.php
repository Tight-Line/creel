<?php

namespace App\Http\Controllers;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Illuminate\Http\Request;

class DocumentController extends Controller
{
    public function __construct(
        private CreelApiClient $api,
    ) {}

    public function index(string $topicId)
    {
        try {
            $topic = $this->api->getTopic($topicId);
            $documents = $this->api->listDocuments($topicId);
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->route('topics.index');
        }

        return view('documents.index', [
            'topic' => $topic,
            'documents' => $documents,
        ]);
    }

    public function edit(string $id)
    {
        try {
            $document = $this->api->getDocument($id);
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->route('topics.index');
        }

        return view('documents.edit', [
            'document' => $document,
        ]);
    }

    public function update(Request $request, string $id)
    {
        $payload = array_filter(
            $request->only([
                'name',
                'url',
                'author',
                'published_at',
            ]),
            fn ($v) => filled($v),
        );

        try {
            $this->api->updateDocument($id, $payload);
            session()->flash('success', 'Document updated.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->back()->withInput();
        }

        // Redirect back to the document's topic.
        try {
            $document = $this->api->getDocument($id);
            return redirect()->route('documents.index', $document['topic_id'] ?? '');
        } catch (CreelApiException) {
            return redirect()->route('topics.index');
        }
    }
}

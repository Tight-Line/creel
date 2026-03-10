<?php

namespace App\Http\Controllers;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;

class ChunkController extends Controller
{
    public function __construct(
        private CreelApiClient $api,
    ) {}

    public function index(string $documentId)
    {
        try {
            $document = $this->api->getDocument($documentId);
            $topic = $this->api->getTopic($document['topic_id']);

            // Get all chunks (active + summaries).
            $activeChunks = $this->api->getDocumentContext($documentId, true);

            // Get compaction history to find compacted (inactive) source chunks.
            $compactionRecords = $this->api->getCompactionHistory($documentId);

            // Collect compacted source chunk IDs and build a lookup of
            // which summary chunk compacted them.
            $compactedChunkIds = [];
            $compactedBy = []; // chunk_id => summary_chunk_id
            $compactionMeta = []; // summary_chunk_id => record
            foreach ($compactionRecords as $record) {
                $summaryId = $record['summary_chunk_id'] ?? '';
                $compactionMeta[$summaryId] = $record;
                foreach ($record['source_chunk_ids'] ?? [] as $sourceId) {
                    $compactedChunkIds[] = $sourceId;
                    $compactedBy[$sourceId] = $summaryId;
                }
            }

            // Fetch each compacted source chunk individually (they are inactive
            // so GetContext won't return them).
            $compactedChunks = [];
            foreach (array_unique($compactedChunkIds) as $chunkId) {
                try {
                    $compactedChunks[] = $this->api->getChunk($chunkId);
                } catch (CreelApiException) {
                    // Chunk may have been deleted; skip it.
                }
            }

            // Tag chunks with their role for the view.
            $allChunks = [];
            $summaryChunkIds = array_keys($compactionMeta);

            foreach ($activeChunks as $chunk) {
                $id = $chunk['id'] ?? '';
                $chunk['_role'] = in_array($id, $summaryChunkIds) ? 'summary' : 'active';
                $chunk['_compaction'] = $compactionMeta[$id] ?? null;
                $allChunks[] = $chunk;
            }

            foreach ($compactedChunks as $chunk) {
                $id = $chunk['id'] ?? '';
                $chunk['_role'] = 'compacted';
                $chunk['_compacted_by'] = $compactedBy[$id] ?? '';
                $allChunks[] = $chunk;
            }

            // Sort by sequence, then by role (active/summary first, compacted last).
            usort($allChunks, function ($a, $b) {
                $seqA = $a['sequence'] ?? 0;
                $seqB = $b['sequence'] ?? 0;
                if ($seqA !== $seqB) {
                    return $seqA <=> $seqB;
                }
                $order = ['active' => 0, 'summary' => 1, 'compacted' => 2];
                return ($order[$a['_role']] ?? 9) <=> ($order[$b['_role']] ?? 9);
            });
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->route('topics.index');
        }

        return view('chunks.index', [
            'document' => $document,
            'topic' => $topic,
            'chunks' => $allChunks,
            'compactionRecords' => $compactionRecords,
        ]);
    }
}

@extends('layouts.app')

@section('title', 'Chunks')

@section('content')
    <div class="flex items-center justify-between mb-6">
        <div>
            <h2 class="text-xl font-semibold">Chunks</h2>
            <p class="text-sm text-slate-500 mt-1">
                {{ $document['name'] ?? '' }}
                <span class="text-slate-400 mx-1">/</span>
                {{ $topic['name'] ?? '' }}
            </p>
        </div>
        <a href="{{ route('documents.index', $topic['id'] ?? '') }}"
           class="px-4 py-2 bg-white border border-slate-300 text-slate-700 text-sm font-medium rounded hover:bg-slate-50 transition-colors">
            Back to Documents
        </a>
    </div>

    {{-- Compaction History --}}
    @if (count($compactionRecords) > 0)
        <div class="bg-white rounded shadow mb-6">
            <div class="px-6 py-4 border-b border-slate-200">
                <h3 class="text-sm font-semibold text-slate-700">Compaction History</h3>
            </div>
            <div class="divide-y divide-slate-100">
                @foreach ($compactionRecords as $record)
                    <div class="px-6 py-3 text-sm">
                        <div class="flex items-center gap-3">
                            <span class="inline-block w-2.5 h-2.5 rounded-full bg-blue-400 flex-shrink-0"></span>
                            <div>
                                <span class="font-medium text-slate-700">{{ count($record['source_chunk_ids'] ?? []) }} chunks</span>
                                <span class="text-slate-400 mx-1">&rarr;</span>
                                <span class="font-mono text-xs text-blue-600">{{ Str::limit($record['summary_chunk_id'] ?? '', 12) }}</span>
                                @if (!empty($record['created_by']))
                                    <span class="text-slate-400 ml-2">by {{ $record['created_by'] }}</span>
                                @endif
                                @if (!empty($record['created_at']['seconds']))
                                    <span class="text-slate-400 ml-2">{{ \Carbon\Carbon::createFromTimestamp($record['created_at']['seconds'])->diffForHumans() }}</span>
                                @endif
                            </div>
                        </div>
                    </div>
                @endforeach
            </div>
        </div>
    @endif

    {{-- Chunk List with Filters --}}
    <div x-data="{ show: { active: true, summary: true, compacted: true } }">

        {{-- Filter toggles --}}
        <div class="flex items-center gap-4 mb-4 text-xs text-slate-600">
            <span class="text-slate-400">Show:</span>
            <label class="flex items-center gap-1.5 cursor-pointer select-none">
                <input type="checkbox" x-model="show.active" class="rounded border-slate-300 text-green-500 focus:ring-green-500">
                <span class="inline-block w-2 h-2 rounded-full bg-green-400"></span> Active
            </label>
            <label class="flex items-center gap-1.5 cursor-pointer select-none">
                <input type="checkbox" x-model="show.summary" class="rounded border-slate-300 text-blue-500 focus:ring-blue-500">
                <span class="inline-block w-2 h-2 rounded-full bg-blue-400"></span> Summary
            </label>
            <label class="flex items-center gap-1.5 cursor-pointer select-none">
                <input type="checkbox" x-model="show.compacted" class="rounded border-slate-300 text-slate-500 focus:ring-slate-500">
                <span class="inline-block w-2 h-2 rounded-full bg-slate-300"></span> Compacted
            </label>
        </div>

        {{-- Table --}}
        <div class="bg-white rounded shadow overflow-hidden">
            <table class="w-full text-sm text-left">
                <thead class="bg-slate-50 text-slate-600 uppercase text-xs tracking-wider">
                    <tr>
                        <th class="px-4 py-3 w-10">#</th>
                        <th class="px-4 py-3 w-16">Status</th>
                        <th class="px-4 py-3">Content</th>
                        <th class="px-4 py-3 w-28">Embedding</th>
                        <th class="px-4 py-3 w-28">Metadata</th>
                        <th class="px-4 py-3 w-36">ID</th>
                    </tr>
                </thead>
                <tbody class="divide-y divide-slate-200">
                    @forelse ($chunks as $chunk)
                        @php
                            $role = $chunk['_role'] ?? 'active';
                            $statusColor = match($role) {
                                'active' => 'bg-green-400',
                                'summary' => 'bg-blue-400',
                                'compacted' => 'bg-slate-300',
                                default => 'bg-slate-300',
                            };
                            $statusLabel = match($role) {
                                'active' => 'Active',
                                'summary' => 'Summary',
                                'compacted' => 'Compacted',
                                default => $role,
                            };
                            $rowClass = $role === 'compacted' ? 'bg-slate-50 text-slate-400' : '';
                        @endphp
                        <tr class="{{ $rowClass }} hover:bg-slate-50"
                            id="chunk-{{ $chunk['id'] ?? '' }}"
                            x-show="show.{{ $role }}"
                            x-transition.opacity>
                            <td class="px-4 py-3 text-xs text-slate-500 align-top">
                                {{ $chunk['sequence'] ?? '' }}
                            </td>
                            <td class="px-4 py-3 align-top">
                                <span class="inline-flex items-center gap-1.5 text-xs">
                                    <span class="inline-block w-2 h-2 rounded-full {{ $statusColor }}"></span>
                                    {{ $statusLabel }}
                                </span>
                            </td>
                            <td class="px-4 py-3 align-top">
                                <div class="max-w-xl">
                                    @if ($role === 'summary')
                                        @php
                                            $compaction = $chunk['_compaction'] ?? null;
                                            $sourceCount = $compaction ? count($compaction['source_chunk_ids'] ?? []) : 0;
                                        @endphp
                                        @if ($sourceCount > 0)
                                            <div class="text-xs text-blue-600 mb-1">
                                                Summarizes {{ $sourceCount }} chunk{{ $sourceCount > 1 ? 's' : '' }}
                                            </div>
                                        @endif
                                    @endif

                                    @if ($role === 'compacted' && !empty($chunk['_compacted_by']))
                                        <div class="text-xs text-slate-400 mb-1">
                                            Replaced by
                                            <a href="#chunk-{{ $chunk['_compacted_by'] }}" class="text-blue-500 hover:text-blue-700 font-mono">
                                                {{ Str::limit($chunk['_compacted_by'], 12) }}
                                            </a>
                                        </div>
                                    @endif

                                    <div class="whitespace-pre-wrap text-sm {{ $role === 'compacted' ? 'text-slate-400 line-through decoration-slate-300' : 'text-slate-800' }}">{{ Str::limit($chunk['content'] ?? '', 500) }}</div>
                                </div>
                            </td>
                            <td class="px-4 py-3 text-xs align-top">
                                @if (!empty($chunk['embedding_id']))
                                    <span class="text-green-600" title="{{ $chunk['embedding_id'] }}">Yes</span>
                                @else
                                    <span class="text-slate-300">None</span>
                                @endif
                            </td>
                            <td class="px-4 py-3 text-xs align-top">
                                @if (!empty($chunk['metadata']))
                                    <details class="cursor-pointer">
                                        <summary class="text-blue-600 hover:text-blue-800">View</summary>
                                        <pre class="mt-1 text-xs text-slate-600 bg-slate-50 p-2 rounded max-w-xs overflow-auto">{{ json_encode($chunk['metadata'], JSON_PRETTY_PRINT) }}</pre>
                                    </details>
                                @else
                                    <span class="text-slate-300">&mdash;</span>
                                @endif
                            </td>
                            <td class="px-4 py-3 font-mono text-xs text-slate-400 align-top" title="{{ $chunk['id'] ?? '' }}">
                                {{ Str::limit($chunk['id'] ?? '', 12) }}
                            </td>
                        </tr>
                    @empty
                        <tr>
                            <td colspan="6" class="px-6 py-8 text-center text-slate-400">
                                No chunks found.
                            </td>
                        </tr>
                    @endforelse
                </tbody>
            </table>
        </div>
    </div>
@endsection

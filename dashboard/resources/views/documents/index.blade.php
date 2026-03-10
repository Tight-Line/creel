@extends('layouts.app')

@section('title', 'Documents')

@section('content')
    <div class="flex items-center justify-between mb-6">
        <div>
            <h2 class="text-xl font-semibold">Documents</h2>
            <p class="text-sm text-slate-500 mt-1">Topic: {{ $topic['name'] ?? $topic['slug'] ?? '' }}</p>
        </div>
        <a href="{{ route('topics.index') }}"
           class="px-4 py-2 bg-white border border-slate-300 text-slate-700 text-sm font-medium rounded hover:bg-slate-50 transition-colors">
            Back to Topics
        </a>
    </div>

    <div class="bg-white rounded shadow overflow-hidden">
        <table class="w-full text-sm text-left">
            <thead class="bg-slate-50 text-slate-600 uppercase text-xs tracking-wider">
                <tr>
                    <th class="px-6 py-3">Name</th>
                    <th class="px-6 py-3">Slug</th>
                    <th class="px-6 py-3">URL</th>
                    <th class="px-6 py-3">Author</th>
                    <th class="px-6 py-3">Published</th>
                    <th class="px-6 py-3 text-right">Actions</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-slate-200">
                @forelse ($documents as $doc)
                    <tr class="hover:bg-slate-50">
                        <td class="px-6 py-4 font-medium text-slate-800">{{ $doc['name'] ?? '' }}</td>
                        <td class="px-6 py-4 text-slate-600 font-mono text-xs">{{ $doc['slug'] ?? '' }}</td>
                        <td class="px-6 py-4">
                            @if (!empty($doc['url']))
                                <a href="{{ $doc['url'] }}" target="_blank" class="text-blue-600 hover:text-blue-800 text-xs truncate block max-w-xs">
                                    {{ $doc['url'] }}
                                </a>
                            @else
                                <span class="text-slate-400">&mdash;</span>
                            @endif
                        </td>
                        <td class="px-6 py-4 text-slate-600">{{ $doc['author'] ?? '' }}</td>
                        <td class="px-6 py-4 text-slate-600 text-xs">
                            @if (!empty($doc['published_at']))
                                {{ \Carbon\Carbon::parse($doc['published_at'])->format('Y-m-d') }}
                            @endif
                        </td>
                        <td class="px-6 py-4 text-right space-x-3">
                            <a href="{{ route('chunks.index', $doc['id']) }}"
                               class="text-blue-600 hover:text-blue-800 text-sm font-medium">
                                Chunks
                            </a>
                            <a href="{{ route('documents.edit', $doc['id']) }}"
                               class="text-blue-600 hover:text-blue-800 text-sm font-medium">
                                Edit
                            </a>
                        </td>
                    </tr>
                @empty
                    <tr>
                        <td colspan="6" class="px-6 py-8 text-center text-slate-400">
                            No documents found.
                        </td>
                    </tr>
                @endforelse
            </tbody>
        </table>
    </div>
@endsection

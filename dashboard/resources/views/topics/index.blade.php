@extends('layouts.app')

@section('title', 'Topics')

@section('content')
    <div class="flex items-center justify-between mb-6">
        <h2 class="text-xl font-semibold">Topics</h2>
    </div>

    <div class="bg-white rounded shadow overflow-hidden">
        <table class="w-full text-sm text-left">
            <thead class="bg-slate-50 text-slate-600 uppercase text-xs tracking-wider">
                <tr>
                    <th class="px-6 py-3">Name</th>
                    <th class="px-6 py-3">Slug</th>
                    <th class="px-6 py-3">LLM Config</th>
                    <th class="px-6 py-3">Embedding Config</th>
                    <th class="px-6 py-3">Prompt Config</th>
                    <th class="px-6 py-3 text-right">Actions</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-slate-200">
                @forelse ($topics as $topic)
                    <tr class="hover:bg-slate-50">
                        <td class="px-6 py-4 font-medium text-slate-800">{{ $topic['name'] ?? '' }}</td>
                        <td class="px-6 py-4 text-slate-600 font-mono text-xs">{{ $topic['slug'] ?? '' }}</td>
                        <td class="px-6 py-4">
                            @if (!empty($topic['llm_config_name']))
                                <span class="text-slate-800">{{ $topic['llm_config_name'] }}</span>
                            @elseif (!empty($topic['llm_config_id']))
                                <span class="text-slate-800">{{ $topic['llm_config_id'] }}</span>
                            @else
                                <span class="text-slate-400">Default</span>
                            @endif
                        </td>
                        <td class="px-6 py-4">
                            @if (!empty($topic['embedding_config_name']))
                                <span class="text-slate-800">{{ $topic['embedding_config_name'] }}</span>
                            @elseif (!empty($topic['embedding_config_id']))
                                <span class="text-slate-800">{{ $topic['embedding_config_id'] }}</span>
                            @else
                                <span class="text-slate-400">Default</span>
                            @endif
                        </td>
                        <td class="px-6 py-4">
                            @if (!empty($topic['extraction_prompt_config_name']))
                                <span class="text-slate-800">{{ $topic['extraction_prompt_config_name'] }}</span>
                            @elseif (!empty($topic['extraction_prompt_config_id']))
                                <span class="text-slate-800">{{ $topic['extraction_prompt_config_id'] }}</span>
                            @else
                                <span class="text-slate-400">Default</span>
                            @endif
                        </td>
                        <td class="px-6 py-4 text-right">
                            <a href="{{ route('topics.edit', $topic['id']) }}"
                               class="text-blue-600 hover:text-blue-800 text-sm font-medium">
                                Edit
                            </a>
                        </td>
                    </tr>
                @empty
                    <tr>
                        <td colspan="6" class="px-6 py-8 text-center text-slate-400">
                            No topics found.
                        </td>
                    </tr>
                @endforelse
            </tbody>
        </table>
    </div>
@endsection

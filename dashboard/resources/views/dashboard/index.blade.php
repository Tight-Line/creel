@extends('layouts.app')

@section('title', 'Dashboard')

@section('content')
    @if (!empty($error))
        <div class="mb-6 px-4 py-3 rounded bg-red-100 border border-red-300 text-red-800 text-sm">
            {{ $error }}
        </div>
    @endif

    {{-- Server Health --}}
    <div class="mb-8">
        <h2 class="text-xl font-semibold mb-4">Server Health</h2>
        <div class="bg-white rounded shadow px-6 py-4 inline-flex items-center gap-6 text-sm">
            @if (!empty($health['status']) && $health['status'] === 'SERVING')
                <span class="inline-flex items-center gap-2">
                    <span class="w-2.5 h-2.5 rounded-full bg-green-500"></span>
                    <span class="font-medium text-green-700">Healthy</span>
                </span>
            @else
                <span class="inline-flex items-center gap-2">
                    <span class="w-2.5 h-2.5 rounded-full bg-red-500"></span>
                    <span class="font-medium text-red-700">{{ $health['status'] ?? 'Unknown' }}</span>
                </span>
            @endif

            @if (!empty($health['version']))
                <span class="text-slate-500">Version: <span class="text-slate-800 font-medium">{{ $health['version'] }}</span></span>
            @endif
        </div>
    </div>

    {{-- Stats Grid --}}
    <h2 class="text-xl font-semibold mb-4">Overview</h2>
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
        @php
            $cards = [
                ['label' => 'Topics', 'count' => $counts['topics'] ?? 0, 'route' => 'topics.index'],
                ['label' => 'Documents', 'count' => $counts['documents'] ?? 0, 'route' => 'topics.index'],
                ['label' => 'Chunks', 'count' => $counts['chunks'] ?? 0, 'route' => 'topics.index'],
                ['label' => 'Memories', 'count' => $counts['memories'] ?? 0, 'route' => 'memories.index'],
                ['label' => 'API Keys', 'count' => $counts['api_key_configs'] ?? 0, 'route' => 'config.apikey.index'],
                ['label' => 'LLM Configs', 'count' => $counts['llm_configs'] ?? 0, 'route' => 'config.llm.index'],
                ['label' => 'Embedding Configs', 'count' => $counts['embedding_configs'] ?? 0, 'route' => 'config.embedding.index'],
                ['label' => 'Extraction Prompts', 'count' => $counts['extraction_prompt_configs'] ?? 0, 'route' => 'config.prompt.index'],
                ['label' => 'System Accounts', 'count' => $counts['system_accounts'] ?? 0, 'route' => 'accounts.index'],
            ];
        @endphp

        @foreach ($cards as $card)
            <a href="{{ route($card['route']) }}"
               class="block bg-white rounded shadow p-6 hover:shadow-md transition-shadow">
                <div class="text-3xl font-bold text-slate-800">{{ $card['count'] }}</div>
                <div class="mt-1 text-sm text-slate-500">{{ $card['label'] }}</div>
            </a>
        @endforeach
    </div>
@endsection

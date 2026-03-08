@extends('layouts.app')

@section('title', 'Edit Topic')

@section('content')
    <div class="mb-6">
        <h2 class="text-xl font-semibold">Edit Topic</h2>
    </div>

    <div class="bg-white rounded shadow max-w-2xl">
        <form method="POST" action="{{ route('topics.update', $topic['id']) }}" class="p-6 space-y-5">
            @csrf
            @method('PATCH')

            <div>
                <label for="name" class="block text-sm font-medium text-slate-700 mb-1">Name</label>
                <input type="text" name="name" id="name" value="{{ old('name', $topic['name'] ?? '') }}" required
                       class="w-full rounded border-slate-300 shadow-sm text-sm focus:ring-blue-500 focus:border-blue-500">
            </div>

            <div>
                <label for="description" class="block text-sm font-medium text-slate-700 mb-1">Description</label>
                <input type="text" name="description" id="description" value="{{ old('description', $topic['description'] ?? '') }}"
                       class="w-full rounded border-slate-300 shadow-sm text-sm focus:ring-blue-500 focus:border-blue-500">
            </div>

            <div>
                <label for="llm_config_id" class="block text-sm font-medium text-slate-700 mb-1">LLM Config</label>
                <select name="llm_config_id" id="llm_config_id"
                        class="w-full rounded border-slate-300 shadow-sm text-sm focus:ring-blue-500 focus:border-blue-500">
                    <option value="">Use Default</option>
                    @foreach ($llmConfigs as $cfg)
                        <option value="{{ $cfg['id'] }}"
                            {{ old('llm_config_id', $topic['llm_config_id'] ?? '') === $cfg['id'] ? 'selected' : '' }}>
                            {{ $cfg['name'] }}
                        </option>
                    @endforeach
                </select>
            </div>

            <div>
                <label for="embedding_config_id" class="block text-sm font-medium text-slate-700 mb-1">Embedding Config</label>
                <select name="embedding_config_id" id="embedding_config_id"
                        class="w-full rounded border-slate-300 shadow-sm text-sm focus:ring-blue-500 focus:border-blue-500">
                    <option value="">Use Default</option>
                    @foreach ($embeddingConfigs as $cfg)
                        <option value="{{ $cfg['id'] }}"
                            {{ old('embedding_config_id', $topic['embedding_config_id'] ?? '') === $cfg['id'] ? 'selected' : '' }}>
                            {{ $cfg['name'] }}
                        </option>
                    @endforeach
                </select>
            </div>

            <div>
                <label for="extraction_prompt_config_id" class="block text-sm font-medium text-slate-700 mb-1">Extraction Prompt</label>
                <select name="extraction_prompt_config_id" id="extraction_prompt_config_id"
                        class="w-full rounded border-slate-300 shadow-sm text-sm focus:ring-blue-500 focus:border-blue-500">
                    <option value="">Use Default</option>
                    @foreach ($promptConfigs as $cfg)
                        <option value="{{ $cfg['id'] }}"
                            {{ old('extraction_prompt_config_id', $topic['extraction_prompt_config_id'] ?? '') === $cfg['id'] ? 'selected' : '' }}>
                            {{ $cfg['name'] }}
                        </option>
                    @endforeach
                </select>
                <p class="mt-1 text-xs text-slate-500">Requires an LLM config to be set</p>
            </div>

            <div class="flex items-center gap-3 pt-2">
                <button type="submit"
                        class="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded hover:bg-blue-700 transition-colors">
                    Save Changes
                </button>
                <a href="{{ route('topics.index') }}"
                   class="px-4 py-2 bg-white border border-slate-300 text-slate-700 text-sm font-medium rounded hover:bg-slate-50 transition-colors">
                    Cancel
                </a>
            </div>
        </form>
    </div>
@endsection

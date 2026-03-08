@extends('layouts.app')

@section('title', 'Create Embedding Config')

@section('content')
    <div class="mb-6">
        <h2 class="text-xl font-semibold">Create Embedding Config</h2>
    </div>

    <div class="bg-white rounded shadow p-6 max-w-2xl">
        <form method="POST" action="{{ route('config.embedding.store') }}">
            @csrf

            <div class="mb-4">
                <label for="name" class="block text-sm font-medium text-slate-700 mb-1">Name</label>
                <input type="text"
                       id="name"
                       name="name"
                       value="{{ old('name') }}"
                       required
                       class="w-full px-3 py-2 border border-slate-300 rounded text-sm text-slate-800 focus:outline-none focus:ring-2 focus:ring-slate-500 focus:border-slate-500">
                @error('name')
                    <p class="mt-1 text-xs text-red-600">{{ $message }}</p>
                @enderror
            </div>

            <div class="mb-4">
                <label for="provider" class="block text-sm font-medium text-slate-700 mb-1">Provider</label>
                <input type="text"
                       id="provider"
                       name="provider"
                       value="{{ old('provider') }}"
                       required
                       class="w-full px-3 py-2 border border-slate-300 rounded text-sm text-slate-800 focus:outline-none focus:ring-2 focus:ring-slate-500 focus:border-slate-500">
                @error('provider')
                    <p class="mt-1 text-xs text-red-600">{{ $message }}</p>
                @enderror
            </div>

            <div class="mb-4">
                <label for="model" class="block text-sm font-medium text-slate-700 mb-1">Model</label>
                <input type="text"
                       id="model"
                       name="model"
                       value="{{ old('model') }}"
                       required
                       class="w-full px-3 py-2 border border-slate-300 rounded text-sm text-slate-800 focus:outline-none focus:ring-2 focus:ring-slate-500 focus:border-slate-500">
                @error('model')
                    <p class="mt-1 text-xs text-red-600">{{ $message }}</p>
                @enderror
            </div>

            <div class="mb-4">
                <label for="dimensions" class="block text-sm font-medium text-slate-700 mb-1">Dimensions</label>
                <input type="number"
                       id="dimensions"
                       name="dimensions"
                       value="{{ old('dimensions') }}"
                       required
                       min="1"
                       class="w-full px-3 py-2 border border-slate-300 rounded text-sm text-slate-800 focus:outline-none focus:ring-2 focus:ring-slate-500 focus:border-slate-500">
                @error('dimensions')
                    <p class="mt-1 text-xs text-red-600">{{ $message }}</p>
                @enderror
            </div>

            <div class="mb-4">
                <label for="api_key_config_id" class="block text-sm font-medium text-slate-700 mb-1">API Key Config</label>
                <select id="api_key_config_id"
                        name="api_key_config_id"
                        required
                        class="w-full px-3 py-2 border border-slate-300 rounded text-sm text-slate-800 focus:outline-none focus:ring-2 focus:ring-slate-500 focus:border-slate-500">
                    <option value="">Select an API key config</option>
                    @foreach ($apiKeyConfigs as $akc)
                        <option value="{{ $akc['id'] }}" @selected(old('api_key_config_id') == $akc['id'])>
                            {{ $akc['name'] }}
                        </option>
                    @endforeach
                </select>
                @error('api_key_config_id')
                    <p class="mt-1 text-xs text-red-600">{{ $message }}</p>
                @enderror
            </div>

            <div class="mb-6">
                <label class="inline-flex items-center gap-2">
                    <input type="checkbox"
                           name="is_default"
                           value="1"
                           @checked(old('is_default'))
                           class="rounded border-slate-300 text-blue-600 focus:ring-blue-500">
                    <span class="text-sm text-slate-700">Set as default</span>
                </label>
            </div>

            <div class="flex items-center gap-3">
                <button type="submit"
                        class="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded hover:bg-blue-700 transition-colors">
                    Create
                </button>
                <a href="{{ route('config.embedding.index') }}"
                   class="px-4 py-2 text-sm font-medium text-slate-600 hover:text-slate-800">
                    Cancel
                </a>
            </div>
        </form>
    </div>
@endsection

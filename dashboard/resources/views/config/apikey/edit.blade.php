@extends('layouts.app')

@section('title', 'Edit API Key Config')

@section('content')
    <div class="mb-6">
        <h2 class="text-xl font-semibold">Edit API Key Config</h2>
    </div>

    <div class="bg-white rounded shadow max-w-lg">
        <form method="POST" action="{{ route('config.apikey.update', $config['id']) }}" class="p-6 space-y-5">
            @csrf
            @method('PATCH')

            <div>
                <label for="name" class="block text-sm font-medium text-slate-700 mb-1">Name</label>
                <input type="text"
                       id="name"
                       name="name"
                       value="{{ old('name', $config['name'] ?? '') }}"
                       required
                       class="w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                @error('name')
                    <p class="mt-1 text-sm text-red-600">{{ $message }}</p>
                @enderror
            </div>

            <div>
                <label for="provider" class="block text-sm font-medium text-slate-700 mb-1">Provider</label>
                <input type="text"
                       id="provider"
                       name="provider"
                       value="{{ old('provider', $config['provider'] ?? '') }}"
                       required
                       class="w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                @error('provider')
                    <p class="mt-1 text-sm text-red-600">{{ $message }}</p>
                @enderror
            </div>

            <div>
                <label for="api_key" class="block text-sm font-medium text-slate-700 mb-1">API Key</label>
                <input type="password"
                       id="api_key"
                       name="api_key"
                       placeholder="Leave blank to keep current"
                       class="w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
                @error('api_key')
                    <p class="mt-1 text-sm text-red-600">{{ $message }}</p>
                @enderror
            </div>

            <div class="flex items-center gap-3 pt-2">
                <button type="submit"
                        class="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded hover:bg-blue-700 transition-colors">
                    Save Changes
                </button>
                <a href="{{ route('config.apikey.index') }}"
                   class="px-4 py-2 text-sm font-medium text-slate-600 hover:text-slate-800">
                    Cancel
                </a>
            </div>
        </form>
    </div>
@endsection

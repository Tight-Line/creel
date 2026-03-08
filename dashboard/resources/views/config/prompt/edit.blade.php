@extends('layouts.app')

@section('title', 'Edit Extraction Prompt')

@section('content')
    <div class="mb-6">
        <h2 class="text-xl font-semibold">Edit Extraction Prompt</h2>
    </div>

    <div class="bg-white rounded shadow max-w-2xl">
        <form method="POST" action="{{ route('config.prompt.update', $config['id']) }}" class="p-6 space-y-5">
            @csrf
            @method('PATCH')

            <div>
                <label for="name" class="block text-sm font-medium text-slate-700 mb-1">Name</label>
                <input type="text" name="name" id="name" value="{{ old('name', $config['name'] ?? '') }}" required
                       class="w-full rounded border-slate-300 shadow-sm text-sm focus:ring-blue-500 focus:border-blue-500">
            </div>

            <div>
                <label for="prompt" class="block text-sm font-medium text-slate-700 mb-1">Prompt</label>
                <textarea name="prompt" id="prompt" rows="6" required
                          class="w-full rounded border-slate-300 shadow-sm text-sm focus:ring-blue-500 focus:border-blue-500">{{ old('prompt', $config['prompt'] ?? '') }}</textarea>
            </div>

            <div>
                <label for="description" class="block text-sm font-medium text-slate-700 mb-1">Description</label>
                <input type="text" name="description" id="description" value="{{ old('description', $config['description'] ?? '') }}"
                       class="w-full rounded border-slate-300 shadow-sm text-sm focus:ring-blue-500 focus:border-blue-500">
            </div>

            <div class="flex items-center gap-3 pt-2">
                <button type="submit"
                        class="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded hover:bg-blue-700 transition-colors">
                    Save Changes
                </button>
                <a href="{{ route('config.prompt.index') }}"
                   class="px-4 py-2 bg-white border border-slate-300 text-slate-700 text-sm font-medium rounded hover:bg-slate-50 transition-colors">
                    Cancel
                </a>
            </div>
        </form>
    </div>
@endsection

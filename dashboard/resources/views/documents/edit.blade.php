@extends('layouts.app')

@section('title', 'Edit Document')

@section('content')
    <div class="mb-6">
        <h2 class="text-xl font-semibold">Edit Document</h2>
    </div>

    <div class="bg-white rounded shadow max-w-2xl">
        <form method="POST" action="{{ route('documents.update', $document['id']) }}" class="p-6 space-y-5">
            @csrf
            @method('PATCH')

            <div>
                <label for="name" class="block text-sm font-medium text-slate-700 mb-1">Name</label>
                <input type="text" name="name" id="name" value="{{ old('name', $document['name'] ?? '') }}" required
                       class="w-full rounded border-slate-300 shadow-sm text-sm focus:ring-blue-500 focus:border-blue-500">
            </div>

            <div>
                <label for="url" class="block text-sm font-medium text-slate-700 mb-1">URL</label>
                <input type="url" name="url" id="url" value="{{ old('url', $document['url'] ?? '') }}"
                       placeholder="https://example.com/article"
                       class="w-full rounded border-slate-300 shadow-sm text-sm focus:ring-blue-500 focus:border-blue-500">
            </div>

            <div>
                <label for="author" class="block text-sm font-medium text-slate-700 mb-1">Author</label>
                <input type="text" name="author" id="author" value="{{ old('author', $document['author'] ?? '') }}"
                       class="w-full rounded border-slate-300 shadow-sm text-sm focus:ring-blue-500 focus:border-blue-500">
            </div>

            <div>
                <label for="published_at" class="block text-sm font-medium text-slate-700 mb-1">Published Date</label>
                <input type="date" name="published_at" id="published_at"
                       value="{{ old('published_at', !empty($document['published_at']) ? \Carbon\Carbon::parse($document['published_at'])->format('Y-m-d') : '') }}"
                       class="w-full rounded border-slate-300 shadow-sm text-sm focus:ring-blue-500 focus:border-blue-500">
            </div>

            <div class="flex items-center gap-3 pt-2">
                <button type="submit"
                        class="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded hover:bg-blue-700 transition-colors">
                    Save Changes
                </button>
                <a href="{{ route('documents.index', $document['topic_id'] ?? '') }}"
                   class="px-4 py-2 bg-white border border-slate-300 text-slate-700 text-sm font-medium rounded hover:bg-slate-50 transition-colors">
                    Cancel
                </a>
            </div>
        </form>
    </div>
@endsection

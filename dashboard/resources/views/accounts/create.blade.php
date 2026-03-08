@extends('layouts.app')

@section('title', 'Create System Account')

@section('content')
    <div class="mb-6">
        <h2 class="text-xl font-semibold">Create System Account</h2>
    </div>

    <div class="bg-white rounded shadow max-w-2xl">
        <form method="POST" action="{{ route('accounts.store') }}" class="p-6 space-y-5">
            @csrf

            <div>
                <label for="name" class="block text-sm font-medium text-slate-700 mb-1">Name</label>
                <input type="text" name="name" id="name" value="{{ old('name') }}" required
                       class="w-full rounded border-slate-300 shadow-sm text-sm focus:ring-blue-500 focus:border-blue-500">
            </div>

            <div class="flex items-center gap-3 pt-2">
                <button type="submit"
                        class="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded hover:bg-blue-700 transition-colors">
                    Create
                </button>
                <a href="{{ route('accounts.index') }}"
                   class="px-4 py-2 bg-white border border-slate-300 text-slate-700 text-sm font-medium rounded hover:bg-slate-50 transition-colors">
                    Cancel
                </a>
            </div>
        </form>
    </div>
@endsection

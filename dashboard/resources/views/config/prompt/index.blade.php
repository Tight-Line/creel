@extends('layouts.app')

@section('title', 'Extraction Prompts')

@php use Carbon\Carbon; @endphp

@section('content')
    <div class="flex items-center justify-between mb-6">
        <h2 class="text-xl font-semibold">Extraction Prompts</h2>
        <a href="{{ route('config.prompt.create') }}"
           class="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded hover:bg-blue-700 transition-colors">
            Create
        </a>
    </div>

    <div class="bg-white rounded shadow overflow-hidden">
        <table class="w-full text-sm text-left">
            <thead class="bg-slate-50 text-slate-600 uppercase text-xs tracking-wider">
                <tr>
                    <th class="px-6 py-3">Name</th>
                    <th class="px-6 py-3">Description</th>
                    <th class="px-6 py-3">Default</th>
                    <th class="px-6 py-3">Created</th>
                    <th class="px-6 py-3 text-right">Actions</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-slate-200">
                @forelse ($configs as $config)
                    <tr class="hover:bg-slate-50">
                        <td class="px-6 py-4 font-medium text-slate-800">{{ $config['name'] ?? '' }}</td>
                        <td class="px-6 py-4 text-slate-600">{{ \Illuminate\Support\Str::limit($config['description'] ?? '', 60) }}</td>
                        <td class="px-6 py-4">
                            @if (!empty($config['is_default']))
                                <span class="inline-block px-2 py-0.5 text-xs font-semibold rounded-full bg-green-100 text-green-800">
                                    Default
                                </span>
                            @endif
                        </td>
                        <td class="px-6 py-4 text-slate-500">
                            @if (!empty($config['created_at']['seconds']))
                                {{ Carbon::createFromTimestamp($config['created_at']['seconds'])->format('M j, Y') }}
                            @endif
                        </td>
                        <td class="px-6 py-4 text-right">
                            <div class="inline-flex items-center gap-3">
                                <a href="{{ route('config.prompt.edit', $config['id']) }}"
                                   class="text-blue-600 hover:text-blue-800 text-sm font-medium">
                                    Edit
                                </a>

                                @if (empty($config['is_default']))
                                    <form method="POST" action="{{ route('config.prompt.default', $config['id']) }}">
                                        @csrf
                                        <button type="submit"
                                                class="text-slate-600 hover:text-slate-800 text-sm font-medium">
                                            Set Default
                                        </button>
                                    </form>
                                @endif

                                <form method="POST"
                                      action="{{ route('config.prompt.destroy', $config['id']) }}"
                                      x-data
                                      x-on:submit.prevent="if (confirm('Are you sure you want to delete this extraction prompt?')) $el.submit()">
                                    @csrf
                                    @method('DELETE')
                                    <button type="submit"
                                            class="text-red-600 hover:text-red-800 text-sm font-medium">
                                        Delete
                                    </button>
                                </form>
                            </div>
                        </td>
                    </tr>
                @empty
                    <tr>
                        <td colspan="5" class="px-6 py-8 text-center text-slate-400">
                            No extraction prompts found. Create one to get started.
                        </td>
                    </tr>
                @endforelse
            </tbody>
        </table>
    </div>
@endsection

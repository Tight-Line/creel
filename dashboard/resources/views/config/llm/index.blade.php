@extends('layouts.app')

@section('title', 'LLM Configs')

@php use Carbon\Carbon; @endphp

@section('content')
    <div class="flex items-center justify-between mb-6">
        <h2 class="text-xl font-semibold">LLM Configs</h2>
        <a href="{{ route('config.llm.create') }}"
           class="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded hover:bg-blue-700 transition-colors">
            Create
        </a>
    </div>

    <div class="bg-white rounded shadow overflow-hidden">
        <table class="w-full text-sm text-left">
            <thead class="bg-slate-50 text-slate-600 uppercase text-xs tracking-wider">
                <tr>
                    <th class="px-6 py-3">Name</th>
                    <th class="px-6 py-3">Provider</th>
                    <th class="px-6 py-3">Model</th>
                    <th class="px-6 py-3">API Key Config</th>
                    <th class="px-6 py-3">Default</th>
                    <th class="px-6 py-3">Created</th>
                    <th class="px-6 py-3 text-right">Actions</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-slate-200">
                @forelse ($configs as $config)
                    <tr class="hover:bg-slate-50">
                        <td class="px-6 py-4 font-medium text-slate-800">{{ $config['name'] ?? '' }}</td>
                        <td class="px-6 py-4 text-slate-600">{{ $config['provider'] ?? '' }}</td>
                        <td class="px-6 py-4 text-slate-600">{{ $config['model'] ?? '' }}</td>
                        <td class="px-6 py-4 text-slate-600">
                            {{ $apiKeyMap[$config['api_key_config_id'] ?? ''] ?? '' }}
                        </td>
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
                            <div class="inline-flex items-center gap-2">
                                <a href="{{ route('config.llm.edit', $config['id']) }}"
                                   class="text-slate-400 hover:text-blue-600 transition-colors" title="Edit">
                                    <x-heroicon-o-pencil-square class="w-5 h-5" />
                                </a>

                                @if (empty($config['is_default']))
                                    <form method="POST" action="{{ route('config.llm.default', $config['id']) }}">
                                        @csrf
                                        <button type="submit"
                                                class="text-slate-400 hover:text-green-600 transition-colors" title="Set Default">
                                            <x-heroicon-o-star class="w-5 h-5" />
                                        </button>
                                    </form>
                                @endif

                                <form method="POST"
                                      action="{{ route('config.llm.destroy', $config['id']) }}"
                                      x-data
                                      x-on:submit.prevent="if (confirm('Are you sure you want to delete this LLM config?')) $el.submit()">
                                    @csrf
                                    @method('DELETE')
                                    <button type="submit"
                                            class="text-slate-400 hover:text-red-600 transition-colors" title="Delete">
                                        <x-heroicon-o-trash class="w-5 h-5" />
                                    </button>
                                </form>
                            </div>
                        </td>
                    </tr>
                @empty
                    <tr>
                        <td colspan="7" class="px-6 py-8 text-center text-slate-400">
                            No LLM configs found. Create one to get started.
                        </td>
                    </tr>
                @endforelse
            </tbody>
        </table>
    </div>
@endsection

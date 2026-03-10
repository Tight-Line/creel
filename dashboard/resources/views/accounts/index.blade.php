@extends('layouts.app')

@section('title', 'System Accounts')

@php use Carbon\Carbon; @endphp

@section('content')
    <div class="flex items-center justify-between mb-6">
        <h2 class="text-xl font-semibold">System Accounts</h2>
        <a href="{{ route('accounts.create') }}"
           class="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded hover:bg-blue-700 transition-colors">
            Create
        </a>
    </div>

    {{-- New API key alert --}}
    @if (session('api_key'))
        <div class="mb-6 px-4 py-4 rounded bg-amber-50 border border-amber-300" x-data="{ copied: false }">
            <p class="text-sm font-semibold text-amber-800 mb-2">New API Key</p>
            <p class="text-sm text-amber-700 mb-3">
                Copy this key now. It will not be shown again.
            </p>
            <div class="flex items-center gap-2">
                <code class="block flex-1 px-3 py-2 bg-white border border-amber-200 rounded text-sm font-mono text-slate-800 break-all select-all">{{ session('api_key') }}</code>
                <button type="button"
                        x-on:click="navigator.clipboard.writeText('{{ session('api_key') }}'); copied = true; setTimeout(() => copied = false, 2000)"
                        class="shrink-0 px-3 py-2 bg-amber-600 text-white text-sm font-medium rounded hover:bg-amber-700 transition-colors">
                    <span x-show="!copied">Copy</span>
                    <span x-show="copied" x-cloak>Copied!</span>
                </button>
            </div>
        </div>
    @endif

    <div class="bg-white rounded shadow overflow-hidden">
        <table class="w-full text-sm text-left">
            <thead class="bg-slate-50 text-slate-600 uppercase text-xs tracking-wider">
                <tr>
                    <th class="px-6 py-3">Name</th>
                    <th class="px-6 py-3">Account ID</th>
                    <th class="px-6 py-3">Created</th>
                    <th class="px-6 py-3 text-right">Actions</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-slate-200">
                @forelse ($accounts as $account)
                    <tr class="hover:bg-slate-50">
                        <td class="px-6 py-4 font-medium text-slate-800">{{ $account['name'] ?? '' }}</td>
                        <td class="px-6 py-4 text-slate-600 font-mono text-xs">{{ $account['id'] ?? '' }}</td>
                        <td class="px-6 py-4 text-slate-500">
                            @if (!empty($account['created_at']['seconds']))
                                {{ Carbon::createFromTimestamp($account['created_at']['seconds'])->format('M j, Y') }}
                            @endif
                        </td>
                        <td class="px-6 py-4 text-right">
                            <div class="inline-flex items-center gap-2">
                                {{-- Rotate --}}
                                <form method="POST" action="{{ route('accounts.rotate', $account['id']) }}"
                                      class="inline-flex items-center gap-1">
                                    @csrf
                                    <input type="number" name="grace_period_seconds" placeholder="Grace (sec)"
                                           class="w-24 rounded border-slate-300 shadow-sm text-xs focus:ring-blue-500 focus:border-blue-500">
                                    <button type="submit"
                                            class="text-slate-400 hover:text-blue-600 transition-colors" title="Rotate Key">
                                        <x-heroicon-o-arrow-path class="w-5 h-5" />
                                    </button>
                                </form>

                                {{-- Revoke --}}
                                <form method="POST"
                                      action="{{ route('accounts.revoke', $account['id']) }}"
                                      x-data
                                      x-on:submit.prevent="if (confirm('Are you sure you want to revoke this API key?')) $el.submit()">
                                    @csrf
                                    <button type="submit"
                                            class="text-slate-400 hover:text-amber-600 transition-colors" title="Revoke Key">
                                        <x-heroicon-o-no-symbol class="w-5 h-5" />
                                    </button>
                                </form>

                                {{-- Delete --}}
                                <form method="POST"
                                      action="{{ route('accounts.destroy', $account['id']) }}"
                                      x-data
                                      x-on:submit.prevent="if (confirm('Are you sure you want to delete this system account?')) $el.submit()">
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
                        <td colspan="4" class="px-6 py-8 text-center text-slate-400">
                            No system accounts found. Create one to get started.
                        </td>
                    </tr>
                @endforelse
            </tbody>
        </table>
    </div>
@endsection

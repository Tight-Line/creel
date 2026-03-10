@extends('layouts.app')

@section('title', 'Memories')

@section('content')
    <div class="flex items-center justify-between mb-6">
        <h2 class="text-xl font-semibold">Memory Scopes</h2>
    </div>

    <div class="bg-white rounded shadow overflow-hidden">
        <table class="w-full text-sm text-left">
            <thead class="bg-slate-50 text-slate-600 uppercase text-xs tracking-wider">
                <tr>
                    <th class="px-6 py-3">Scope</th>
                    <th class="px-6 py-3 text-right">Actions</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-slate-200">
                @forelse ($scopes as $scope)
                    <tr class="hover:bg-slate-50">
                        <td class="px-6 py-4 font-mono text-sm text-slate-800">{{ $scope }}</td>
                        <td class="px-6 py-4 text-right">
                            <a href="{{ route('memories.scope', $scope) }}"
                               class="text-slate-400 hover:text-slate-700 transition-colors" title="Browse">
                                <x-heroicon-o-eye class="w-5 h-5 inline" />
                            </a>
                        </td>
                    </tr>
                @empty
                    <tr>
                        <td colspan="2" class="px-6 py-8 text-center text-slate-400">
                            No memory scopes found.
                        </td>
                    </tr>
                @endforelse
            </tbody>
        </table>
    </div>
@endsection

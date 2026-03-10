@extends('layouts.app')

@section('title', 'Memories: ' . $scope)

@section('content')
    <div class="flex items-center justify-between mb-6">
        <div>
            <a href="{{ route('memories.index') }}" class="text-sm text-blue-600 hover:text-blue-800">&larr; All Scopes</a>
            <h2 class="text-xl font-semibold mt-1">Scope: <span class="font-mono">{{ $scope }}</span></h2>
        </div>
    </div>

    <div class="bg-white rounded shadow overflow-hidden">
        <table class="w-full text-sm text-left">
            <thead class="bg-slate-50 text-slate-600 uppercase text-xs tracking-wider">
                <tr>
                    <th class="px-6 py-3">Content</th>
                    <th class="px-6 py-3">Subject</th>
                    <th class="px-6 py-3">Predicate</th>
                    <th class="px-6 py-3">Object</th>
                    <th class="px-6 py-3">Status</th>
                    <th class="px-6 py-3">Created</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-slate-200">
                @forelse ($memories as $memory)
                    <tr class="hover:bg-slate-50">
                        <td class="px-6 py-4 text-slate-800 max-w-md truncate">{{ $memory['content'] ?? '' }}</td>
                        <td class="px-6 py-4 text-slate-600">{{ $memory['subject'] ?? '' }}</td>
                        <td class="px-6 py-4 text-slate-600">{{ $memory['predicate'] ?? '' }}</td>
                        <td class="px-6 py-4 text-slate-600">{{ $memory['object'] ?? '' }}</td>
                        <td class="px-6 py-4">
                            @php
                                $status = $memory['status'] ?? 'unknown';
                                $color = match ($status) {
                                    'active' => 'bg-green-100 text-green-800',
                                    'invalidated' => 'bg-red-100 text-red-800',
                                    default => 'bg-slate-100 text-slate-800',
                                };
                            @endphp
                            <span class="inline-block px-2 py-0.5 rounded text-xs font-medium {{ $color }}">
                                {{ $status }}
                            </span>
                        </td>
                        <td class="px-6 py-4 text-slate-500 text-xs">{{ $memory['created_at'] ?? '' }}</td>
                    </tr>
                @empty
                    <tr>
                        <td colspan="6" class="px-6 py-8 text-center text-slate-400">
                            No memories in this scope.
                        </td>
                    </tr>
                @endforelse
            </tbody>
        </table>
    </div>
@endsection

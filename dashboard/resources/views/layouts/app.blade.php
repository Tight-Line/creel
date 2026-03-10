<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>@yield('title', 'Creel Dashboard')</title>
    @vite(['resources/css/app.css', 'resources/js/app.js'])
</head>
<body class="bg-slate-100 text-slate-800 min-h-screen flex">

    {{-- Sidebar --}}
    <aside class="w-64 bg-slate-800 text-slate-200 min-h-screen flex flex-col justify-between">
        <div>
            <div class="px-6 py-5 border-b border-slate-700">
                <h1 class="text-lg font-semibold text-white tracking-wide">Creel</h1>
            </div>

            <nav class="mt-4 space-y-1 px-3">
                @php
                    $links = [
                        ['url' => '/', 'label' => 'Dashboard', 'pattern' => '/'],
                        ['url' => '/config/apikey', 'label' => 'API Keys', 'pattern' => 'config/apikey*'],
                        ['url' => '/config/llm', 'label' => 'LLM Configs', 'pattern' => 'config/llm*'],
                        ['url' => '/config/embedding', 'label' => 'Embedding Configs', 'pattern' => 'config/embedding*'],
                        ['url' => '/config/prompt', 'label' => 'Extraction Prompts', 'pattern' => 'config/prompt*'],
                        ['url' => '/topics', 'label' => 'Topics', 'pattern' => 'topics*'],
                        ['url' => '/memories', 'label' => 'Memories', 'pattern' => 'memories*'],
                        ['url' => '/accounts', 'label' => 'System Accounts', 'pattern' => 'accounts*'],
                    ];
                @endphp

                @foreach ($links as $link)
                    @php
                        $isActive = $link['pattern'] === '/'
                            ? request()->is('/')
                            : request()->is($link['pattern']);
                    @endphp
                    <a href="{{ $link['url'] }}"
                       class="block px-3 py-2 rounded text-sm font-medium transition-colors
                              {{ $isActive
                                  ? 'bg-slate-700 text-white'
                                  : 'text-slate-300 hover:bg-slate-700 hover:text-white' }}">
                        {{ $link['label'] }}
                    </a>
                @endforeach
            </nav>
        </div>

        <div class="px-3 pb-5">
            <form method="POST" action="/logout">
                @csrf
                <button type="submit"
                        class="w-full text-left px-3 py-2 rounded text-sm font-medium text-slate-400 hover:bg-slate-700 hover:text-white transition-colors">
                    Sign Out
                </button>
            </form>
        </div>
    </aside>

    {{-- Main content --}}
    <div class="flex-1 flex flex-col">
        {{-- Flash messages --}}
        @if (session('success'))
            <div class="mx-6 mt-4 px-4 py-3 rounded bg-green-100 border border-green-300 text-green-800 text-sm">
                {{ session('success') }}
            </div>
        @endif

        @if (session('error'))
            <div class="mx-6 mt-4 px-4 py-3 rounded bg-red-100 border border-red-300 text-red-800 text-sm">
                {{ session('error') }}
            </div>
        @endif

        <main class="flex-1 p-6">
            @yield('content')
        </main>
    </div>

</body>
</html>

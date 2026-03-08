<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Sign In - Creel Dashboard</title>
    @vite(['resources/css/app.css'])
</head>
<body class="bg-slate-100 min-h-screen flex items-center justify-center">

    <div class="w-full max-w-sm">
        <div class="bg-white rounded-lg shadow-md p-8">
            <h1 class="text-xl font-semibold text-slate-800 text-center mb-6">Creel Dashboard</h1>

            @if ($errors->has('credentials'))
                <div class="mb-4 px-4 py-3 rounded bg-red-100 border border-red-300 text-red-800 text-sm">
                    {{ $errors->first('credentials') }}
                </div>
            @endif

            <form method="POST" action="/login">
                @csrf

                <div class="mb-4">
                    <label for="username" class="block text-sm font-medium text-slate-700 mb-1">Username</label>
                    <input type="text"
                           id="username"
                           name="username"
                           value="{{ old('username') }}"
                           required
                           autofocus
                           class="w-full px-3 py-2 border border-slate-300 rounded text-sm text-slate-800 focus:outline-none focus:ring-2 focus:ring-slate-500 focus:border-slate-500">
                    @error('username')
                        <p class="mt-1 text-xs text-red-600">{{ $message }}</p>
                    @enderror
                </div>

                <div class="mb-6">
                    <label for="password" class="block text-sm font-medium text-slate-700 mb-1">Password</label>
                    <input type="password"
                           id="password"
                           name="password"
                           required
                           class="w-full px-3 py-2 border border-slate-300 rounded text-sm text-slate-800 focus:outline-none focus:ring-2 focus:ring-slate-500 focus:border-slate-500">
                    @error('password')
                        <p class="mt-1 text-xs text-red-600">{{ $message }}</p>
                    @enderror
                </div>

                <button type="submit"
                        class="w-full bg-slate-800 text-white py-2 px-4 rounded text-sm font-medium hover:bg-slate-700 transition-colors focus:outline-none focus:ring-2 focus:ring-slate-500 focus:ring-offset-2">
                    Sign In
                </button>
            </form>
        </div>
    </div>

</body>
</html>

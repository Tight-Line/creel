<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;

class LoginController extends Controller
{
    public function showForm()
    {
        return view('auth.login');
    }

    public function login(Request $request)
    {
        $request->validate([
            'username' => 'required|string',
            'password' => 'required|string',
        ]);

        $username = config('creel.dashboard_username');
        $password = config('creel.dashboard_password');

        if ($request->input('username') === $username && $request->input('password') === $password) {
            $request->session()->put('authenticated', true);
            return redirect('/');
        }

        return redirect()->back()->withErrors(['credentials' => 'Invalid username or password.']);
    }

    public function logout(Request $request)
    {
        $request->session()->invalidate();
        $request->session()->regenerateToken();

        return redirect('/login');
    }
}

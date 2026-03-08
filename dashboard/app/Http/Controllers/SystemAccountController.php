<?php

namespace App\Http\Controllers;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Illuminate\Http\Request;

class SystemAccountController extends Controller
{
    public function __construct(
        private CreelApiClient $api,
    ) {}

    public function index()
    {
        try {
            $accounts = $this->api->listSystemAccounts();
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            $accounts = [];
        }

        return view('accounts.index', ['accounts' => $accounts]);
    }

    public function create()
    {
        return view('accounts.create');
    }

    public function store(Request $request)
    {
        $request->validate([
            'name' => 'required|string',
        ]);

        try {
            $result = $this->api->createSystemAccount($request->only(['name']));
            session()->flash('success', 'System account created.');
            session()->flash('api_key', $result['api_key'] ?? $result['key'] ?? '');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
            return redirect()->back()->withInput();
        }

        return redirect()->route('accounts.index');
    }

    public function rotate(Request $request, string $id)
    {
        $gracePeriod = (int) $request->input('grace_period_seconds', 0);

        try {
            $result = $this->api->rotateKey($id, $gracePeriod);
            session()->flash('success', 'API key rotated.');
            session()->flash('api_key', $result['api_key'] ?? $result['key'] ?? '');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
        }

        return redirect()->route('accounts.index');
    }

    public function revoke(string $id)
    {
        try {
            $this->api->revokeKey($id);
            session()->flash('success', 'API key revoked.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
        }

        return redirect()->route('accounts.index');
    }

    public function destroy(string $id)
    {
        try {
            $this->api->deleteSystemAccount($id);
            session()->flash('success', 'System account deleted.');
        } catch (CreelApiException $e) {
            session()->flash('error', $e->getMessage());
        }

        return redirect()->route('accounts.index');
    }
}

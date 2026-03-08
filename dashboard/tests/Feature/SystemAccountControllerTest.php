<?php

namespace Tests\Feature;

use App\Exceptions\CreelApiException;
use App\Services\CreelApiClient;
use Mockery;
use Tests\TestCase;

class SystemAccountControllerTest extends TestCase
{
    private $api;

    protected function setUp(): void
    {
        parent::setUp();
        $this->api = Mockery::mock(CreelApiClient::class);
        $this->app->instance(CreelApiClient::class, $this->api);
    }

    private function authed()
    {
        return $this->withSession(['authenticated' => true]);
    }

    public function test_index_lists_accounts(): void
    {
        $accounts = [['id' => '1', 'name' => 'bot-account']];
        $this->api->shouldReceive('listSystemAccounts')->once()->andReturn($accounts);

        $response = $this->authed()->get('/accounts');

        $response->assertStatus(200);
        $response->assertViewIs('accounts.index');
        $response->assertViewHas('accounts', $accounts);
    }

    public function test_index_handles_api_error(): void
    {
        $this->api->shouldReceive('listSystemAccounts')
            ->andThrow(new CreelApiException(500, '', 'Error'));

        $response = $this->authed()->get('/accounts');

        $response->assertStatus(200);
        $response->assertSessionHas('error');
        $response->assertViewHas('accounts', []);
    }

    public function test_create_returns_form(): void
    {
        $response = $this->authed()->get('/accounts/create');

        $response->assertStatus(200);
        $response->assertViewIs('accounts.create');
    }

    public function test_store_creates_account_and_flashes_api_key(): void
    {
        $this->api->shouldReceive('createSystemAccount')
            ->once()
            ->with(['name' => 'new-bot'])
            ->andReturn(['id' => '1', 'api_key' => 'creel_ak_new123']);

        $response = $this->authed()->post('/accounts', ['name' => 'new-bot']);

        $response->assertRedirect(route('accounts.index'));
        $response->assertSessionHas('success');
        $response->assertSessionHas('api_key', 'creel_ak_new123');
    }

    public function test_store_flashes_key_field_as_fallback(): void
    {
        $this->api->shouldReceive('createSystemAccount')
            ->once()
            ->andReturn(['id' => '1', 'key' => 'creel_ak_fallback']);

        $response = $this->authed()->post('/accounts', ['name' => 'bot']);

        $response->assertSessionHas('api_key', 'creel_ak_fallback');
    }

    public function test_store_validates_name_required(): void
    {
        $response = $this->authed()->post('/accounts', []);

        $response->assertSessionHasErrors(['name']);
    }

    public function test_store_handles_api_error(): void
    {
        $this->api->shouldReceive('createSystemAccount')
            ->andThrow(new CreelApiException(409, '', 'Name taken'));

        $response = $this->authed()->post('/accounts', ['name' => 'dup']);

        $response->assertRedirect();
        $response->assertSessionHas('error', 'Name taken');
    }

    public function test_rotate_sends_grace_period_and_flashes_key(): void
    {
        $this->api->shouldReceive('rotateKey')
            ->once()
            ->with('1', 3600)
            ->andReturn(['api_key' => 'creel_ak_rotated']);

        $response = $this->authed()->post('/accounts/1/rotate', [
            'grace_period_seconds' => '3600',
        ]);

        $response->assertRedirect(route('accounts.index'));
        $response->assertSessionHas('success');
        $response->assertSessionHas('api_key', 'creel_ak_rotated');
    }

    public function test_rotate_defaults_grace_period_to_zero(): void
    {
        $this->api->shouldReceive('rotateKey')
            ->once()
            ->with('1', 0)
            ->andReturn(['api_key' => 'creel_ak_rotated']);

        $response = $this->authed()->post('/accounts/1/rotate');

        $response->assertRedirect(route('accounts.index'));
    }

    public function test_rotate_handles_api_error(): void
    {
        $this->api->shouldReceive('rotateKey')
            ->andThrow(new CreelApiException(404, '', 'Not found'));

        $response = $this->authed()->post('/accounts/1/rotate');

        $response->assertRedirect(route('accounts.index'));
        $response->assertSessionHas('error');
    }

    public function test_revoke(): void
    {
        $this->api->shouldReceive('revokeKey')->with('1')->once();

        $response = $this->authed()->post('/accounts/1/revoke');

        $response->assertRedirect(route('accounts.index'));
        $response->assertSessionHas('success');
    }

    public function test_revoke_handles_api_error(): void
    {
        $this->api->shouldReceive('revokeKey')
            ->andThrow(new CreelApiException(404, '', 'Not found'));

        $response = $this->authed()->post('/accounts/1/revoke');

        $response->assertRedirect(route('accounts.index'));
        $response->assertSessionHas('error');
    }

    public function test_destroy(): void
    {
        $this->api->shouldReceive('deleteSystemAccount')->with('1')->once();

        $response = $this->authed()->delete('/accounts/1');

        $response->assertRedirect(route('accounts.index'));
        $response->assertSessionHas('success');
    }

    public function test_destroy_handles_api_error(): void
    {
        $this->api->shouldReceive('deleteSystemAccount')
            ->andThrow(new CreelApiException(500, '', 'Error'));

        $response = $this->authed()->delete('/accounts/1');

        $response->assertRedirect(route('accounts.index'));
        $response->assertSessionHas('error');
    }
}

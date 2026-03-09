<?php

namespace Tests\Feature;

use Tests\TestCase;

class AuthTest extends TestCase
{
    protected function defineEnvironment($app): void
    {
        $app['env'] = 'testing';
    }

    protected function setUp(): void
    {
        parent::setUp();
        config([
            'creel.dashboard_username' => 'testuser',
            'creel.dashboard_password' => 'testpass',
        ]);
    }

    public function test_login_page_renders(): void
    {
        $response = $this->get('/login');

        $response->assertStatus(200);
        $response->assertViewIs('auth.login');
    }

    public function test_login_with_valid_credentials_redirects_to_dashboard(): void
    {
        $response = $this->post('/login', [
            'username' => 'testuser',
            'password' => 'testpass',
        ]);

        $response->assertRedirect('/');
        $this->assertTrue(session('authenticated'));
    }

    public function test_login_with_invalid_credentials_redirects_back_with_errors(): void
    {
        $response = $this->post('/login', [
            'username' => 'wrong',
            'password' => 'wrong',
        ]);

        $response->assertRedirect();
        $response->assertSessionHasErrors('credentials');
        $this->assertNull(session('authenticated'));
    }

    public function test_login_validates_required_fields(): void
    {
        $response = $this->post('/login', []);

        $response->assertSessionHasErrors(['username', 'password']);
    }

    public function test_logout_clears_session_and_redirects_to_login(): void
    {
        $response = $this->withSession(['authenticated' => true])
            ->post('/logout');

        $response->assertRedirect('/login');
        $this->assertNull(session('authenticated'));
    }

    public function test_unauthenticated_user_is_redirected_to_login(): void
    {
        $response = $this->get('/');

        $response->assertRedirect(route('login'));
    }

    public function test_authenticated_user_can_access_protected_routes(): void
    {
        // Mock the API client so the dashboard doesn't make real HTTP calls
        $this->mockApiClient();

        $response = $this->withSession(['authenticated' => true])
            ->get('/');

        $response->assertStatus(200);
    }

    private function mockApiClient(): void
    {
        $mock = \Mockery::mock(\App\Services\CreelApiClient::class);
        $mock->shouldReceive('health')->andReturn(['status' => 'ok']);
        $mock->shouldReceive('listApiKeyConfigs')->andReturn([]);
        $mock->shouldReceive('listLlmConfigs')->andReturn([]);
        $mock->shouldReceive('listEmbeddingConfigs')->andReturn([]);
        $mock->shouldReceive('listPromptConfigs')->andReturn([]);
        $mock->shouldReceive('listTopics')->andReturn([]);
        $mock->shouldReceive('listSystemAccounts')->andReturn([]);
        $this->app->instance(\App\Services\CreelApiClient::class, $mock);
    }
}

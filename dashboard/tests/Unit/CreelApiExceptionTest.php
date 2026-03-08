<?php

namespace Tests\Unit;

use App\Exceptions\CreelApiException;
use PHPUnit\Framework\TestCase;

class CreelApiExceptionTest extends TestCase
{
    public function test_stores_status_code_and_body(): void
    {
        $e = new CreelApiException(404, '{"message":"not found"}');

        $this->assertSame(404, $e->statusCode);
        $this->assertSame('{"message":"not found"}', $e->errorBody);
    }

    public function test_uses_custom_message_when_provided(): void
    {
        $e = new CreelApiException(500, 'raw body', 'Custom message');

        $this->assertSame('Custom message', $e->getMessage());
    }

    public function test_formats_default_message_from_status_and_body(): void
    {
        $e = new CreelApiException(422, 'validation failed');

        $this->assertSame('Creel API error (422): validation failed', $e->getMessage());
    }

    public function test_extends_runtime_exception(): void
    {
        $e = new CreelApiException(500, '');

        $this->assertInstanceOf(\RuntimeException::class, $e);
    }
}

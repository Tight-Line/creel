<?php

namespace App\Exceptions;

use RuntimeException;

class CreelApiException extends RuntimeException
{
    public function __construct(
        public readonly int $statusCode,
        public readonly string $errorBody,
        string $message = '',
    ) {
        parent::__construct($message ?: "Creel API error ({$statusCode}): {$errorBody}");
    }
}

<?php

// Override Docker env vars that conflict with testing
$_ENV['APP_KEY'] = 'base64:YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE=';
$_ENV['APP_ENV'] = 'testing';
$_ENV['SESSION_DRIVER'] = 'array';

$_SERVER['APP_KEY'] = $_ENV['APP_KEY'];
$_SERVER['APP_ENV'] = $_ENV['APP_ENV'];
$_SERVER['SESSION_DRIVER'] = $_ENV['SESSION_DRIVER'];

putenv('APP_KEY=' . $_ENV['APP_KEY']);
putenv('APP_ENV=' . $_ENV['APP_ENV']);
putenv('SESSION_DRIVER=' . $_ENV['SESSION_DRIVER']);

require __DIR__ . '/../vendor/autoload.php';

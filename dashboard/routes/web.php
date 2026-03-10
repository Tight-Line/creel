<?php

use App\Http\Controllers\ApiKeyConfigController;
use App\Http\Controllers\DashboardController;
use App\Http\Controllers\EmbeddingConfigController;
use App\Http\Controllers\ExtractionPromptController;
use App\Http\Controllers\LlmConfigController;
use App\Http\Controllers\LoginController;
use App\Http\Controllers\SystemAccountController;
use App\Http\Controllers\DocumentController;
use App\Http\Controllers\MemoryController;
use App\Http\Controllers\TopicController;
use Illuminate\Support\Facades\Route;

// Auth
Route::get('/login', [LoginController::class, 'showForm'])->name('login');
Route::post('/login', [LoginController::class, 'login']);
Route::post('/logout', [LoginController::class, 'logout'])->name('logout');

// Authenticated routes
Route::middleware('auth.dashboard')->group(function () {
    Route::get('/', [DashboardController::class, 'index'])->name('dashboard');

    // API Key Configs
    Route::get('/config/apikey', [ApiKeyConfigController::class, 'index'])->name('config.apikey.index');
    Route::get('/config/apikey/create', [ApiKeyConfigController::class, 'create'])->name('config.apikey.create');
    Route::post('/config/apikey', [ApiKeyConfigController::class, 'store'])->name('config.apikey.store');
    Route::get('/config/apikey/{id}/edit', [ApiKeyConfigController::class, 'edit'])->name('config.apikey.edit');
    Route::patch('/config/apikey/{id}', [ApiKeyConfigController::class, 'update'])->name('config.apikey.update');
    Route::delete('/config/apikey/{id}', [ApiKeyConfigController::class, 'destroy'])->name('config.apikey.destroy');
    Route::post('/config/apikey/{id}/default', [ApiKeyConfigController::class, 'setDefault'])->name('config.apikey.default');

    // LLM Configs
    Route::get('/config/llm', [LlmConfigController::class, 'index'])->name('config.llm.index');
    Route::get('/config/llm/create', [LlmConfigController::class, 'create'])->name('config.llm.create');
    Route::post('/config/llm', [LlmConfigController::class, 'store'])->name('config.llm.store');
    Route::get('/config/llm/{id}/edit', [LlmConfigController::class, 'edit'])->name('config.llm.edit');
    Route::patch('/config/llm/{id}', [LlmConfigController::class, 'update'])->name('config.llm.update');
    Route::delete('/config/llm/{id}', [LlmConfigController::class, 'destroy'])->name('config.llm.destroy');
    Route::post('/config/llm/{id}/default', [LlmConfigController::class, 'setDefault'])->name('config.llm.default');

    // Embedding Configs
    Route::get('/config/embedding', [EmbeddingConfigController::class, 'index'])->name('config.embedding.index');
    Route::get('/config/embedding/create', [EmbeddingConfigController::class, 'create'])->name('config.embedding.create');
    Route::post('/config/embedding', [EmbeddingConfigController::class, 'store'])->name('config.embedding.store');
    Route::get('/config/embedding/{id}/edit', [EmbeddingConfigController::class, 'edit'])->name('config.embedding.edit');
    Route::patch('/config/embedding/{id}', [EmbeddingConfigController::class, 'update'])->name('config.embedding.update');
    Route::delete('/config/embedding/{id}', [EmbeddingConfigController::class, 'destroy'])->name('config.embedding.destroy');
    Route::post('/config/embedding/{id}/default', [EmbeddingConfigController::class, 'setDefault'])->name('config.embedding.default');

    // Extraction Prompt Configs
    Route::get('/config/prompt', [ExtractionPromptController::class, 'index'])->name('config.prompt.index');
    Route::get('/config/prompt/create', [ExtractionPromptController::class, 'create'])->name('config.prompt.create');
    Route::post('/config/prompt', [ExtractionPromptController::class, 'store'])->name('config.prompt.store');
    Route::get('/config/prompt/{id}/edit', [ExtractionPromptController::class, 'edit'])->name('config.prompt.edit');
    Route::patch('/config/prompt/{id}', [ExtractionPromptController::class, 'update'])->name('config.prompt.update');
    Route::delete('/config/prompt/{id}', [ExtractionPromptController::class, 'destroy'])->name('config.prompt.destroy');
    Route::post('/config/prompt/{id}/default', [ExtractionPromptController::class, 'setDefault'])->name('config.prompt.default');

    // Topics
    Route::get('/topics', [TopicController::class, 'index'])->name('topics.index');
    Route::get('/topics/{id}/edit', [TopicController::class, 'edit'])->name('topics.edit');
    Route::patch('/topics/{id}', [TopicController::class, 'update'])->name('topics.update');

    // Documents
    Route::get('/topics/{topicId}/documents', [DocumentController::class, 'index'])->name('documents.index');
    Route::get('/documents/{id}/edit', [DocumentController::class, 'edit'])->name('documents.edit');
    Route::patch('/documents/{id}', [DocumentController::class, 'update'])->name('documents.update');

    // Memories
    Route::get('/memories', [MemoryController::class, 'index'])->name('memories.index');
    Route::get('/memories/{scope}', [MemoryController::class, 'scope'])->name('memories.scope');

    // System Accounts
    Route::get('/accounts', [SystemAccountController::class, 'index'])->name('accounts.index');
    Route::get('/accounts/create', [SystemAccountController::class, 'create'])->name('accounts.create');
    Route::post('/accounts', [SystemAccountController::class, 'store'])->name('accounts.store');
    Route::post('/accounts/{id}/rotate', [SystemAccountController::class, 'rotate'])->name('accounts.rotate');
    Route::post('/accounts/{id}/revoke', [SystemAccountController::class, 'revoke'])->name('accounts.revoke');
    Route::delete('/accounts/{id}', [SystemAccountController::class, 'destroy'])->name('accounts.destroy');
});

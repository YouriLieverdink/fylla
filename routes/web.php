<?php

use App\Http\Controllers\IssueController;
use Illuminate\Support\Facades\Route;
use Inertia\Inertia;

Route::get('/', [IssueController::class, 'index']);
Route::post('/sync', [IssueController::class, 'sync'])->name('sync');

Route::get('/playground', fn () => Inertia::render('Playground'))->name('playground');

<?php

use App\Http\Controllers\IssueController;
use App\Http\Controllers\TimerController;
use Illuminate\Support\Facades\Route;
use Inertia\Inertia;

Route::get('/', [IssueController::class, 'index']);
Route::post('/sync', [IssueController::class, 'sync'])->name('sync');

Route::post('/timers', [TimerController::class, 'start'])->name('timers.start');
Route::post('/timers/pause', [TimerController::class, 'pause'])->name('timers.pause');
Route::post('/timers/resume', [TimerController::class, 'resume'])->name('timers.resume');
Route::post('/timers/stop', [TimerController::class, 'stop'])->name('timers.stop');
Route::post('/timers/notes', [TimerController::class, 'note'])->name('timers.notes');

Route::get('/playground', fn () => Inertia::render('Playground'))->name('playground');

<?php

use App\Http\Controllers\CapacityController;
use App\Http\Controllers\IssueController;
use App\Http\Controllers\ProjectController;
use App\Http\Controllers\PullRequestController;
use App\Http\Controllers\TimerController;
use App\Http\Controllers\UtilizationController;
use Illuminate\Support\Facades\Route;
use Inertia\Inertia;

Route::get('/', [IssueController::class, 'index']);
Route::post('/sync', [IssueController::class, 'sync'])->name('sync');

Route::get('/projects', [ProjectController::class, 'index'])->name('projects.index');
Route::patch('/projects/{project}', [ProjectController::class, 'update'])->name('projects.update');

Route::get('/utilization', [UtilizationController::class, 'index'])->name('utilization.index');

Route::get('/capacity', [CapacityController::class, 'index'])->name('capacity.index');
Route::post('/capacity', [CapacityController::class, 'store'])->name('capacity.store');
Route::post('/capacity/accrual', [CapacityController::class, 'accrual'])->name('capacity.accrual');
Route::patch('/capacity/{capacityAdjustment}', [CapacityController::class, 'update'])->name('capacity.update');
Route::delete('/capacity/{capacityAdjustment}', [CapacityController::class, 'destroy'])->name('capacity.destroy');

Route::get('/kendo/issues/search', [PullRequestController::class, 'candidates'])->name('kendo.issues.search');
Route::post('/pull-requests/{pullRequest}/resolve', [PullRequestController::class, 'resolve'])->name('pull-requests.resolve');
Route::post('/pull-requests/{pullRequest}/timer', [PullRequestController::class, 'startTimer'])->name('pull-requests.timer');

Route::post('/timers', [TimerController::class, 'start'])->name('timers.start');
Route::post('/timers/pause', [TimerController::class, 'pause'])->name('timers.pause');
Route::post('/timers/resume', [TimerController::class, 'resume'])->name('timers.resume');
Route::post('/timers/stop', [TimerController::class, 'stop'])->name('timers.stop');
Route::post('/timers/start-time', [TimerController::class, 'startTime'])->name('timers.start-time');
Route::post('/timers/notes', [TimerController::class, 'note'])->name('timers.notes');

Route::get('/playground', fn () => Inertia::render('Playground'))->name('playground');

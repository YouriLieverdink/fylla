<?php

use App\Http\Controllers\CapacityController;
use App\Http\Controllers\ClientContextController;
use App\Http\Controllers\ClientController;
use App\Http\Controllers\DeliveryController;
use App\Http\Controllers\DraftController;
use App\Http\Controllers\EstimationController;
use App\Http\Controllers\IssueController;
use App\Http\Controllers\ProjectController;
use App\Http\Controllers\PullRequestController;
use App\Http\Controllers\SettingsController;
use App\Http\Controllers\TimerController;
use App\Http\Controllers\UtilizationController;
use Illuminate\Support\Facades\Route;
use Inertia\Inertia;

Route::get('/', [IssueController::class, 'index']);
Route::patch('/issues/{issue}', [IssueController::class, 'update'])->name('issues.update');
Route::post('/sync', [IssueController::class, 'sync'])->name('sync');

Route::post('/drafts', [DraftController::class, 'store'])->name('drafts.store');
Route::patch('/drafts/{draft}', [DraftController::class, 'update'])->name('drafts.update');
Route::post('/drafts/{draft}/promote', [DraftController::class, 'promote'])->name('drafts.promote');
Route::delete('/drafts/{draft}', [DraftController::class, 'destroy'])->name('drafts.destroy');

Route::get('/clients', [ProjectController::class, 'index'])->name('clients.index');
Route::patch('/projects/{project}', [ProjectController::class, 'update'])->name('projects.update');
Route::post('/clients', [ClientController::class, 'store'])->name('clients.store');
Route::patch('/clients/{client}', [ClientController::class, 'update'])->name('clients.update');
Route::delete('/clients/{client}', [ClientController::class, 'destroy'])->name('clients.destroy');

Route::get('/delivery', [DeliveryController::class, 'index'])->name('delivery.index');
Route::get('/delivery/{client}', [ClientContextController::class, 'show'])->name('delivery.show');

Route::get('/utilization', [UtilizationController::class, 'index'])->name('utilization.index');

Route::get('/estimation', [EstimationController::class, 'index'])->name('estimation.index');

Route::get('/capacity', [CapacityController::class, 'index'])->name('capacity.index');
Route::post('/capacity', [CapacityController::class, 'store'])->name('capacity.store');
Route::post('/capacity/accrual', [CapacityController::class, 'accrual'])->name('capacity.accrual');
Route::patch('/capacity/{capacityAdjustment}', [CapacityController::class, 'update'])->name('capacity.update');
Route::delete('/capacity/{capacityAdjustment}', [CapacityController::class, 'destroy'])->name('capacity.destroy');

Route::get('/kendo/issues/search', [PullRequestController::class, 'candidates'])->name('kendo.issues.search');
Route::post('/pull-requests/{pullRequest}/resolve', [PullRequestController::class, 'resolve'])->name('pull-requests.resolve');
Route::post('/pull-requests/{pullRequest}/timer', [PullRequestController::class, 'startTimer'])->name('pull-requests.timer');

Route::post('/timers', [TimerController::class, 'start'])->name('timers.start');
Route::post('/timers/adhoc', [TimerController::class, 'adhoc'])->name('timers.adhoc');
Route::post('/timers/pause', [TimerController::class, 'pause'])->name('timers.pause');
Route::post('/timers/resume', [TimerController::class, 'resume'])->name('timers.resume');
Route::post('/timers/stop', [TimerController::class, 'stop'])->name('timers.stop');
Route::post('/timers/start-time', [TimerController::class, 'startTime'])->name('timers.start-time');
Route::post('/timers/notes', [TimerController::class, 'note'])->name('timers.notes');

Route::get('/settings', [SettingsController::class, 'edit'])->name('settings.edit');
Route::put('/settings', [SettingsController::class, 'update'])->name('settings.update');

Route::get('/playground', fn () => Inertia::render('Playground'))->name('playground');

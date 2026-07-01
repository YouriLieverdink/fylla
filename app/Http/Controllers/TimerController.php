<?php

namespace App\Http\Controllers;

use App\Models\Issue;
use App\Services\TimerService;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;

class TimerController extends Controller
{
    public function __construct(private readonly TimerService $timers) {}

    public function start(Request $request): RedirectResponse
    {
        $issue = Issue::findOrFail($request->integer('issue_id'));
        $this->timers->start($issue);

        return back();
    }

    public function pause(): RedirectResponse
    {
        $this->timers->pause();

        return back();
    }

    public function resume(): RedirectResponse
    {
        $this->timers->resume();

        return back();
    }

    public function stop(): RedirectResponse
    {
        $this->timers->stop();

        return back();
    }

    public function comment(Request $request): RedirectResponse
    {
        $this->timers->comment((string) $request->input('comment', ''));

        return back();
    }
}

<?php

namespace App\Events;

use Illuminate\Broadcasting\Channel;
use Illuminate\Contracts\Broadcasting\ShouldBroadcastNow;
use Illuminate\Contracts\Broadcasting\ShouldRescue;
use Illuminate\Foundation\Events\Dispatchable;

/**
 * Signal-only push for the activity surface (#92): "something changed", never
 * *what* changed. Listeners answer with a partial Inertia reload, so moment
 * grouping, failedCount and canRetry stay shaped in the controller.
 *
 * ShouldBroadcastNow, not ShouldBroadcast: a queued broadcast is itself a queue
 * job, which JobRunRecorder would record, which would emit again — the loop.
 * `BroadcastManager::queue()` routes Now through `dispatchNow()`, so no job row
 * exists to capture. ShouldRescue keeps a Reverb outage from failing the sync
 * job that emitted — a dead socket makes the page stale, not the sync broken.
 */
class ActivityChanged implements ShouldBroadcastNow, ShouldRescue
{
    use Dispatchable;

    public function broadcastOn(): Channel
    {
        // Public: this app has no auth, so no /broadcasting/auth is in play.
        return new Channel('activity');
    }

    /** Named explicitly so the client isn't coupled to Echo's namespace guess. */
    public function broadcastAs(): string
    {
        return 'activity.changed';
    }
}

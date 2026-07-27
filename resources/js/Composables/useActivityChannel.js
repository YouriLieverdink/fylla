import { onUnmounted, ref } from 'vue';
import { router } from '@inertiajs/vue3';
import { echo } from '../echo';

/**
 * Subscribe a surface to the activity signal (#92), replacing what used to be a
 * 1s `usePoll`. The event is signal-only, so the answer is always the same:
 * reload the props this surface owns and let the controller re-shape them.
 *
 * `online` drives the offline dot. There is no poll floor behind this — a dead
 * socket means the page is frozen, so it has to say so.
 *
 * @param {string[]} only Inertia partial-reload keys this surface depends on.
 */
export function useActivityChannel(only) {
    const online = ref(false);
    if (!echo) return { online };

    const handler = () => router.reload({ only });
    const channel = echo.channel('activity');
    channel.listen('.activity.changed', handler);

    // Optimistic: only a genuinely-down state lights the tell, so the ~200ms
    // handshake on a cold load doesn't flash "offline" at every page open.
    const down = ['unavailable', 'failed', 'disconnected'];
    const connection = echo.connector.pusher.connection;
    online.value = !down.includes(connection.state);
    const onStateChange = ({ current }) => (online.value = !down.includes(current));
    connection.bind('state_change', onStateChange);

    onUnmounted(() => {
        // Drop this callback only — never leaveChannel(). The page and its
        // header both subscribe, and an Inertia visit mounts the next page's
        // header before unmounting the old one, so unsubscribing here would
        // tear down the subscription the incoming page just reused.
        channel.stopListening('.activity.changed', handler);
        connection.unbind('state_change', onStateChange);
    });

    return { online };
}

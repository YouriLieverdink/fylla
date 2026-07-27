import Echo from 'laravel-echo';
import Pusher from 'pusher-js';

/**
 * The app's one Echo instance, or null when Reverb isn't configured (#92).
 *
 * A module export rather than `window.Echo` so components import it explicitly
 * and tests can `vi.mock('../echo')`. Built eagerly from the `reverb` shared
 * prop in app.js — available there before any component mounts, so there is no
 * boot-order dance.
 */
export let echo = null;

export function createEcho({ key, port } = {}) {
    // No key configured: skip Echo entirely. Every page still renders and the
    // offline tell stays lit, rather than an eager throw blanking the app.
    if (!key) return;

    echo = new Echo({
        broadcaster: 'reverb',
        key,
        // The publisher's REVERB_HOST is container-local (127.0.0.1); the
        // browser has to come back to whatever host it loaded the page from.
        wsHost: window.location.hostname,
        wsPort: port,
        wssPort: port,
        forceTLS: window.location.protocol === 'https:',
        enabledTransports: ['ws', 'wss'],
        Pusher,
    });
}

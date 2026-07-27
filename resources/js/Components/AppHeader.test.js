import { describe, it, expect, beforeEach, vi } from 'vitest';
import { shallowMount } from '@vue/test-utils';

const { reload } = vi.hoisted(() => ({ reload: vi.fn() }));
vi.mock('@inertiajs/vue3', () => ({
    router: { reload, post: vi.fn() },
    Link: { template: '<a><slot /></a>' },
    usePage: () => ({ url: '/', props: { lastSyncedAt: null, activityRunning: false, activityFailures: 0 } }),
}));

// Relative, not '@/echo' — this repo has no '@' alias.
const { echo, listeners } = vi.hoisted(() => {
    const listeners = {};
    return {
        listeners,
        echo: {
            channel: () => ({
                listen: (name, cb) => (listeners[name] = cb),
                stopListening: vi.fn(),
            }),
            connector: { pusher: { connection: { state: 'connected', bind: vi.fn(), unbind: vi.fn() } } },
        },
    };
});
vi.mock('../echo', () => ({ echo }));

import AppHeader from './AppHeader.vue';

describe('AppHeader live updates', () => {
    beforeEach(() => reload.mockClear());

    it('an activity signal reloads all three header props', () => {
        const w = shallowMount(AppHeader);

        expect(reload).not.toHaveBeenCalled();
        listeners['.activity.changed']();

        // All three, or "Sync now" stops clearing its timestamp and failure dot.
        expect(reload).toHaveBeenCalledWith({
            only: ['lastSyncedAt', 'activityRunning', 'activityFailures'],
        });
        w.unmount();
    });
});

describe('AppHeader without a configured socket', () => {
    it('mounts and reloads nothing when the echo module is null', async () => {
        vi.resetModules();
        vi.doMock('../echo', () => ({ echo: null }));
        const { default: Offline } = await import('./AppHeader.vue');

        reload.mockClear();
        delete listeners['.activity.changed'];
        const w = shallowMount(Offline);

        expect(w.exists()).toBe(true);
        expect(listeners['.activity.changed']).toBeUndefined(); // never subscribed
        expect(reload).not.toHaveBeenCalled();
        w.unmount();
    });
});

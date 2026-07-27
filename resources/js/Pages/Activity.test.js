import { describe, it, expect, beforeEach, vi } from 'vitest';
import { shallowMount } from '@vue/test-utils';

const { reload } = vi.hoisted(() => ({ reload: vi.fn() }));
vi.mock('@inertiajs/vue3', () => ({
    router: { reload, post: vi.fn() },
    Link: { template: '<a><slot /></a>' },
    usePage: () => ({ url: '/activity', props: {} }),
}));

// Relative, not '@/echo' — this repo has no '@' alias.
const { echo, listeners } = vi.hoisted(() => {
    const listeners = {};
    const connection = {
        state: 'connected',
        bind: (_e, cb) => (listeners.stateChange = cb),
        unbind: vi.fn(),
    };
    return {
        listeners,
        echo: {
            channel: () => ({
                listen: (name, cb) => (listeners[name] = cb),
                stopListening: vi.fn(),
            }),
            connector: { pusher: { connection } },
        },
    };
});
vi.mock('../echo', () => ({ echo }));

import Activity from './Activity.vue';

const moments = [
    {
        id: 'm1',
        trigger: 'manual',
        status: 'ok',
        failedCount: 0,
        startedAt: '2026-07-23T09:00:00Z',
        runs: [{ id: 1, jobClass: 'App\\Jobs\\SyncKendoIssues', status: 'ok', startedAt: '2026-07-23T09:00:00Z', finishedAt: '2026-07-23T09:00:01Z' }],
    },
];

describe('Activity live updates', () => {
    beforeEach(() => reload.mockClear());

    it('an activity signal reloads only the moments prop', () => {
        const w = shallowMount(Activity, { props: { moments } });

        expect(reload).not.toHaveBeenCalled();
        listeners['.activity.changed']();

        expect(reload).toHaveBeenCalledWith({ only: ['moments'] });
        w.unmount();
    });

    // There is no poll floor behind the socket, so a dropped connection has to
    // be visible — otherwise the page just quietly stops being true.
    it('losing the connection reveals the offline tell', async () => {
        const w = shallowMount(Activity, { props: { moments } });
        expect(w.text()).not.toContain('offline');

        listeners.stateChange({ current: 'unavailable' });
        await w.vm.$nextTick();

        expect(w.text()).toContain('offline');
        w.unmount();
    });
});

describe('Activity without a configured socket', () => {
    it('mounts and reloads nothing when the echo module is null', async () => {
        vi.resetModules();
        vi.doMock('../echo', () => ({ echo: null }));
        const { default: Offline } = await import('./Activity.vue');

        reload.mockClear();
        delete listeners['.activity.changed'];
        const w = shallowMount(Offline, { props: { moments } });

        expect(w.exists()).toBe(true);
        expect(listeners['.activity.changed']).toBeUndefined(); // never subscribed
        expect(reload).not.toHaveBeenCalled();
        w.unmount();
    });
});

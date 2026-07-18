import { describe, it, expect, beforeEach, vi } from 'vitest';
import { nextTick } from 'vue';
import { shallowMount } from '@vue/test-utils';

const { post, patch, del, visit, usePoll } = vi.hoisted(() => ({
    post: vi.fn(), patch: vi.fn(), del: vi.fn(), visit: vi.fn(), usePoll: vi.fn(),
}));
vi.mock('@inertiajs/vue3', () => ({ router: { post, patch, delete: del, visit }, usePoll }));

import Worklist from './Worklist.vue';
import { registry } from '../Composables/useAction';

// jsdom implements neither; the cursor's scroll-into-view watcher would throw.
Element.prototype.scrollIntoView = vi.fn();
window.scrollTo = vi.fn();

const issue = { kind: 'issue', id: 1, title: 'Fix bug', key: 'ABC-1', priority: 'High', kendo_url: 'http://k/1', score: 5 };
const draft = { kind: 'draft', id: 2, title: 'Email client' };

const run = (id, ...args) => registry.get(id).run(...args);

// focusTargets = [utilization card, timer card, ...items] → jump-3 = first row.
async function mountAt(row, items = [issue, draft]) {
    const w = shallowMount(Worklist, { props: { items } });
    await nextTick();
    if (row) run(`cursor:jump-${row}`);
    return w;
}

describe('Worklist keyset (#44)', () => {
    beforeEach(() => { registry.clear(); vi.clearAllMocks(); });

    it('t starts a timer on the cursor row; a no-op while the cursor is unset', async () => {
        const w = await mountAt(0);
        run('wl:timer'); // cursor unset
        expect(post).not.toHaveBeenCalled();
        run('cursor:jump-3'); // onto the issue row
        run('wl:timer');
        expect(post).toHaveBeenCalledWith('/timers', { issue_id: 1 }, { preserveScroll: true });
        w.unmount();
    });

    it('d is confirm-gated: deletes the draft only when confirmed', async () => {
        const w = await mountAt(4); // draft row
        window.confirm = vi.fn(() => false); // jsdom leaves confirm undefined
        run('wl:done');
        expect(del).not.toHaveBeenCalled();
        window.confirm.mockReturnValue(true);
        run('wl:done');
        expect(del).toHaveBeenCalledWith('/drafts/2', { preserveScroll: true });
        w.unmount();
    });

    it('registers the full keyset under the worklist scope', async () => {
        const w = await mountAt(0);
        const keys = [...registry.values()].filter((a) => a.scope === 'worklist').map((a) => a.keys).sort();
        expect(keys).toEqual(['a', 'c', 'd', 'e', 'm', 'n', 'o', 'p', 'r', 's', 't', 'u']);
        w.unmount();
    });
});

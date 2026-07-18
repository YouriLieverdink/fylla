import { describe, it, expect, beforeEach, vi } from 'vitest';
import { nextTick } from 'vue';
import { shallowMount } from '@vue/test-utils';

import Utilization from './Utilization.vue';
import { registry } from '../Composables/useAction';

// jsdom implements neither; the cursor's scroll-into-view watcher would throw.
Element.prototype.scrollIntoView = vi.fn();
window.scrollTo = vi.fn();

const report = { weeks: [], totals: {}, target: 90, softFloor: 80 };
const run = (id) => registry.get(id).run();

describe('Utilization view-switcher keyset (#45)', () => {
    beforeEach(() => { registry.clear(); });

    it('w/p/t switch the active view', async () => {
        const w = shallowMount(Utilization, { props: { report } });
        await nextTick();

        run('util:projects');
        expect(w.vm.view).toBe('By project');
        run('util:entries');
        expect(w.vm.view).toBe('Time entries');
        run('util:weekly');
        expect(w.vm.view).toBe('Weekly breakdown');

        w.unmount();
    });

    it('registers under the utilization scope', async () => {
        const w = shallowMount(Utilization, { props: { report } });
        await nextTick();
        const keys = [...registry.values()].filter((a) => a.scope === 'utilization').map((a) => a.keys).sort();
        expect(keys).toEqual(['p', 't', 'w']);
        w.unmount();
    });
});

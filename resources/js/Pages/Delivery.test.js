import { describe, it, expect, beforeEach, vi } from 'vitest';
import { nextTick } from 'vue';
import { shallowMount } from '@vue/test-utils';

const { patch } = vi.hoisted(() => ({ patch: vi.fn() }));
vi.mock('@inertiajs/vue3', () => ({ router: { patch, post: vi.fn(), delete: vi.fn() } }));

import Delivery from './Delivery.vue';
import ProjectRow from '../Components/ProjectRow.vue';
import { registry } from '../Composables/useAction';

// jsdom implements neither; the cursor's scroll-into-view watcher would throw.
Element.prototype.scrollIntoView = vi.fn();
window.scrollTo = vi.fn();

const clients = [{ id: 1, initials: 'AC', name: 'Acme', meta: '', hours: 0, target: 40, series: [] }];
const projects = [
    { id: 10, name: 'Assigned proj', code: 'AP', billable: true, client_id: 1 },
    { id: 20, name: 'Unassigned proj', code: 'UP', billable: false, client_id: null },
];
const run = (id) => registry.get(id).run();

describe('Delivery By-project toggle + keybindings (#64)', () => {
    beforeEach(() => {
        registry.clear();
        patch.mockClear();
    });

    it('c/p switch between the By-client cards and the flat project list', async () => {
        const w = shallowMount(Delivery, { props: { clients, projects }, global: { stubs: { Card: false } } });
        await nextTick();

        expect(w.vm.view).toBe('By client');
        expect(w.findAllComponents(ProjectRow)).toHaveLength(0);

        run('delivery:by-project');
        await nextTick();
        expect(w.vm.view).toBe('By project');
        // Flat list shows all projects (assigned + unassigned).
        expect(w.findAllComponents(ProjectRow)).toHaveLength(2);

        run('delivery:by-client');
        expect(w.vm.view).toBe('By client');

        w.unmount();
    });

    it('flat row toggles billable via PATCH /projects/{id}', async () => {
        const w = shallowMount(Delivery, { props: { clients, projects }, global: { stubs: { Card: false } } });
        run('delivery:by-project');
        await nextTick();

        w.findAllComponents(ProjectRow)[1].vm.$emit('toggle-billable', true);

        expect(patch).toHaveBeenCalledWith('/projects/20', { billable: true }, { preserveScroll: true });
        w.unmount();
    });

    it('registers c/p under the delivery scope', async () => {
        const w = shallowMount(Delivery, { props: { clients, projects }, global: { stubs: { Card: false } } });
        await nextTick();
        const keys = [...registry.values()].filter((a) => a.scope === 'delivery').map((a) => a.keys).sort();
        expect(keys).toEqual(['c', 'p']);
        w.unmount();
    });

    it('j/k cursor tracks the active view', async () => {
        const w = shallowMount(Delivery, { props: { clients, projects }, global: { stubs: { Card: false } } });
        await nextTick();

        // By client: cursor walks the projection cards.
        w.vm.cursor.move(1);
        expect(w.vm.cursor.activeKey.value).toBe('d-1');

        run('delivery:by-project');
        await nextTick();
        // Same cursor now resolves against the flat project rows.
        expect(w.vm.cursor.activeKey.value).toBe('pr-10');
        w.vm.cursor.move(1);
        expect(w.vm.cursor.activeKey.value).toBe('pr-20');

        w.unmount();
    });
});

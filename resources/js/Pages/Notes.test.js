import { describe, it, expect, beforeEach, vi } from 'vitest';
import { nextTick } from 'vue';
import { shallowMount } from '@vue/test-utils';

const { get } = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock('@inertiajs/vue3', () => ({ router: { get } }));

import Notes from './Notes.vue';
import MultiSelectFilter from '../Components/MultiSelectFilter.vue';
import { registry } from '../Composables/useAction';

// jsdom implements neither; the cursor's scroll-into-view watcher would throw.
Element.prototype.scrollIntoView = vi.fn();
window.scrollTo = vi.fn();

const rows = [
    { id: 1, date: '2026-07-01', developer: 'Youri', issueKey: 'A-1', issueTitle: 'One', note: 'first', minutes: 60 },
    { id: 2, date: '2026-07-02', developer: 'Youri', issueKey: 'A-2', issueTitle: 'Two', note: 'second', minutes: 30 },
];
const filters = { q: '', clients: [], projects: [], developers: [], from: null, to: null };
const props = { rows, total: 2, filters, clients: [{ id: 7, name: 'Acme' }], projects: [], developers: [] };

const run = (id) => registry.get(id).run();
const mountPage = (extra = {}) =>
    shallowMount(Notes, { props, global: { stubs: { Card: false } }, ...extra });

describe('Notes keybindings + multi-select filters', () => {
    beforeEach(() => {
        registry.clear();
        get.mockClear();
    });

    it('j/k cursor moves through the note rows', async () => {
        const w = mountPage();
        await nextTick();

        expect(w.findAll('tbody tr')[0].classes()).not.toContain('ring-accent');

        run('cursor:down');
        await nextTick();
        expect(w.findAll('tbody tr')[0].classes()).toContain('ring-accent');

        run('cursor:down');
        await nextTick();
        expect(w.findAll('tbody tr')[1].classes()).toContain('ring-accent');

        w.unmount();
    });

    it('s focuses the search field', async () => {
        const w = mountPage({ attachTo: document.body });
        await nextTick();

        run('notes:search');
        expect(document.activeElement).toBe(w.find('input[type="search"]').element);

        w.unmount();
    });

    it('multi-select change reloads /notes with array filters', async () => {
        vi.useFakeTimers();
        const w = mountPage();
        await nextTick();

        w.findAllComponents(MultiSelectFilter)[0].vm.$emit('update:modelValue', [7]);
        await nextTick();
        vi.advanceTimersByTime(300);

        expect(get).toHaveBeenCalledWith('/notes', { clients: [7] }, { preserveState: true, preserveScroll: true });

        vi.useRealTimers();
        w.unmount();
    });
});

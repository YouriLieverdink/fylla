import { describe, it, expect, beforeEach } from 'vitest';
import { nextTick } from 'vue';
import { mount } from '@vue/test-utils';
import CheatSheet from './CheatSheet.vue';
import { registry, registerAction } from '../Composables/useAction';

const action = (over = {}) => ({
    id: 'a', label: 'Sync now', keys: '.', scope: 'global', run: () => {}, ...over,
});

// Open via the registered toggle's run() — the same path the layout's tinykeys
// listener invokes on '?'.
function open() {
    registry.get('help').run({ preventDefault: () => {} });
    return nextTick();
}

describe('CheatSheet', () => {
    beforeEach(() => registry.clear());

    it('? opens, Escape closes', async () => {
        const w = mount(CheatSheet);
        expect(w.find('input').exists()).toBe(false);
        await open();
        expect(w.find('input').exists()).toBe(true);
        await w.find('div.fixed').trigger('keydown.esc');
        expect(w.find('input').exists()).toBe(false);
    });

    it('? inside the overlay closes instead of typing into search', async () => {
        const w = mount(CheatSheet);
        await open();
        const event = { key: '?', preventDefault: () => {} };
        await w.find('input').trigger('keydown', event);
        expect(w.find('input').exists()).toBe(false);
    });

    it('lists live registry bindings grouped by scope', async () => {
        const w = mount(CheatSheet);
        registerAction(action());
        registerAction(action({ id: 'b', label: 'Capacity', keys: 'g c', scope: 'nav' }));
        await open();
        const text = w.text();
        expect(text).toContain('Sync now');
        expect(text).toContain('Capacity');
        expect(text).toContain('global');
        expect(text).toContain('nav');
        // Split-key rendering: 'g c' becomes two kbd tokens.
        expect(w.findAll('kbd').map((k) => k.text())).toEqual(expect.arrayContaining(['g', 'c']));
    });

    it('search filters by text', async () => {
        const w = mount(CheatSheet);
        registerAction(action());
        registerAction(action({ id: 'b', label: 'Capacity', keys: 'g c' }));
        await open();
        await w.find('input').setValue('capac');
        expect(w.text()).toContain('Capacity');
        expect(w.text()).not.toContain('Sync now');
    });

    it('shows a static Navigation section, not per-digit registry rows', async () => {
        const w = mount(CheatSheet);
        // navigation-scope entries (the cursor keys) must not render individually.
        registerAction(action({ id: 'cursor:jump-1', label: 'Jump to row 1', keys: '1', scope: 'navigation' }));
        await open();
        expect(w.text()).toContain('Navigation');
        expect(w.text()).toContain('Move cursor / scroll page');
        // A leaked navigation-scope group would render a lowercase 'navigation' header.
        expect(w.text()).not.toContain('navigation');
    });
});

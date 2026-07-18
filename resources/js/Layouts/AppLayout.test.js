import { describe, it, expect, beforeEach, vi } from 'vitest';
import { defineComponent, h, nextTick } from 'vue';
import { mount } from '@vue/test-utils';

const { post } = vi.hoisted(() => ({ post: vi.fn() }));
vi.mock('@inertiajs/vue3', () => ({ router: { post } }));

import AppLayout from './AppLayout.vue';
import { useAction, registry } from '../Composables/useAction';

// tinykeys ignores events missing `code` (its isKeyboardEvent guard), so both
// key and code must be set for the synthetic keystroke to match.
const press = (key, code) => window.dispatchEvent(new KeyboardEvent('keydown', { key, code }));

describe('AppLayout keybinding wiring', () => {
    beforeEach(() => { registry.clear(); post.mockClear(); });

    it('. flows through the registry to Sync now', () => {
        const wrapper = mount(AppLayout);
        press('.', 'Period');
        expect(post).toHaveBeenCalledWith('/sync', {}, { preserveScroll: true });
        wrapper.unmount();
    });

    it('rebinds tinykeys when a component registers a new action', async () => {
        const run = vi.fn();
        const Child = defineComponent({
            setup() { useAction({ id: 'x', label: 'X', keys: 'x', scope: 'global', run }); return () => h('div'); },
        });
        const wrapper = mount(AppLayout, { slots: { default: () => h(Child) } });
        await nextTick();
        press('x', 'KeyX');
        expect(run).toHaveBeenCalled();
        wrapper.unmount();
    });
});

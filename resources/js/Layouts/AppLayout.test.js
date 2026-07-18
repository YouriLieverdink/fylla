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

    // Focus-guard tests (#39). Dispatching from an element (bubbling to the
    // window listener) makes event.target the element, which press() — firing
    // on window — can't do.
    const mountWith = async (action) => {
        const Child = defineComponent({
            setup() { useAction(action); return () => h('div'); },
        });
        const wrapper = mount(AppLayout, { slots: { default: () => h(Child) } });
        await nextTick();
        return wrapper;
    };
    const pressFrom = (el, key, code) =>
        el.dispatchEvent(new KeyboardEvent('keydown', { key, code, bubbles: true }));

    it('suppresses bindings while focused in an input', async () => {
        const run = vi.fn();
        const wrapper = await mountWith({ id: 'x', label: 'X', keys: 'x', scope: 'global', run });
        const input = document.body.appendChild(document.createElement('input'));
        pressFrom(input, 'x', 'KeyX');
        expect(run).not.toHaveBeenCalled();
        input.remove();
        wrapper.unmount();
    });

    it('Escape still fires while focused in an editable context', async () => {
        const run = vi.fn();
        const wrapper = await mountWith({ id: 'esc', label: 'Esc', keys: 'Escape', scope: 'global', run });
        const input = document.body.appendChild(document.createElement('input'));
        pressFrom(input, 'Escape', 'Escape');
        expect(run).toHaveBeenCalled();
        input.remove();
        wrapper.unmount();
    });

    it('data-kb-ignore suppresses bindings for keystrokes within it', async () => {
        const run = vi.fn();
        const wrapper = await mountWith({ id: 'x', label: 'X', keys: 'x', scope: 'global', run });
        const box = document.body.appendChild(document.createElement('div'));
        box.setAttribute('data-kb-ignore', '');
        const inner = box.appendChild(document.createElement('button'));
        pressFrom(inner, 'x', 'KeyX');
        expect(run).not.toHaveBeenCalled();
        box.remove();
        wrapper.unmount();
    });
});

import { describe, it, expect, beforeEach } from 'vitest';
import { defineComponent, h, nextTick, watch } from 'vue';
import { mount } from '@vue/test-utils';
import { useAction, registry, registerAction, unregisterAction } from './useAction';

const action = (over = {}) => ({
    id: 'a', label: 'A', keys: 'a', scope: 'global', run: () => {}, ...over,
});

describe('useAction', () => {
    beforeEach(() => registry.clear());

    it('registers on mount, unregisters on unmount', () => {
        const Comp = defineComponent({
            setup() { useAction(action()); return () => h('div'); },
        });
        const wrapper = mount(Comp);
        expect(registry.has('a')).toBe(true);
        wrapper.unmount();
        expect(registry.has('a')).toBe(false);
    });

    it('registry mutation triggers a watcher (layout rebind path)', async () => {
        let hits = 0;
        const stop = watch(registry, () => hits++);
        registerAction(action());
        await nextTick();
        expect(hits).toBe(1);
        unregisterAction('a');
        await nextTick();
        expect(hits).toBe(2);
        stop();
    });

    it('bound-only: omitting keys is unsupported', () => {
        expect(() => useAction(action({ keys: undefined }))).toThrow();
    });
});

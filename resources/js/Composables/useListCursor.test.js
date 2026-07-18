import { describe, it, expect, beforeEach } from 'vitest';
import { defineComponent, h, nextTick, ref } from 'vue';
import { mount } from '@vue/test-utils';
import { useListCursor } from './useListCursor';
import { registry } from './useAction';

// Drive the cursor inside a real component so useAction's mount/unmount lifecycle
// runs. `items` is a ref the test mutates to simulate re-sort/sync.
function harness(initial) {
    const items = ref(initial);
    let cursor;
    const Comp = defineComponent({
        setup() {
            cursor = useListCursor(() => items.value, (it) => it.kind + '-' + it.id);
            return () => h('div');
        },
    });
    const wrapper = mount(Comp);
    return { items, cursor, wrapper };
}

const row = (kind, id) => ({ kind, id });

describe('useListCursor', () => {
    beforeEach(() => registry.clear());

    it('is unset & invisible on load', () => {
        const { cursor } = harness([row('issue', 1), row('issue', 2)]);
        expect(cursor.index.value).toBe(null);
        expect(cursor.current.value).toBe(null);
        expect(cursor.activeKey.value).toBe(null);
    });

    it('first j/k lands on row 1; j/k then move and clamp at ends', () => {
        const { cursor } = harness([row('issue', 1), row('issue', 2)]);
        cursor.move(-1); // first press → row 1 regardless of direction
        expect(cursor.index.value).toBe(0);
        cursor.move(-1); // clamp at top
        expect(cursor.index.value).toBe(0);
        cursor.move(1);
        expect(cursor.index.value).toBe(1);
        cursor.move(1); // clamp at bottom
        expect(cursor.index.value).toBe(1);
        expect(cursor.current.value).toEqual(row('issue', 2));
    });

    it('registers j, k and digit keys as navigation actions', () => {
        harness([row('issue', 1)]);
        expect(registry.get('cursor:down').keys).toBe('j');
        expect(registry.get('cursor:up').keys).toBe('k');
        expect(registry.get('cursor:jump-1').keys).toBe('1');
        expect(registry.get('cursor:jump-9').keys).toBe('9');
        expect(registry.get('cursor:down').scope).toBe('navigation');
    });

    it('digits 1–9 jump to visible rows; out-of-range is a no-op', () => {
        const { cursor } = harness([row('issue', 1), row('issue', 2), row('issue', 3)]);
        cursor.jump(3);
        expect(cursor.index.value).toBe(2);
        cursor.jump(4); // no such row
        expect(cursor.index.value).toBe(2); // unchanged
    });

    it('tracks the same item id across re-sort', async () => {
        const { items, cursor } = harness([row('issue', 1), row('issue', 2), row('issue', 3)]);
        cursor.jump(2); // on issue-2
        expect(cursor.current.value).toEqual(row('issue', 2));
        items.value = [row('issue', 2), row('issue', 1), row('issue', 3)]; // resort: issue-2 to top
        await nextTick();
        expect(cursor.index.value).toBe(0);
        expect(cursor.current.value).toEqual(row('issue', 2));
    });

    it('clamped-index fallback when the tracked item leaves', async () => {
        const { items, cursor } = harness([row('issue', 1), row('issue', 2), row('issue', 3)]);
        cursor.jump(3); // on issue-3, index 2
        items.value = [row('issue', 1), row('issue', 2)]; // issue-3 removed
        await nextTick();
        expect(cursor.index.value).toBe(1); // clamped into new bounds
        expect(cursor.current.value).toEqual(row('issue', 2));
    });

    it('unsets when the list empties', async () => {
        const { items, cursor } = harness([row('issue', 1)]);
        cursor.move(1);
        expect(cursor.index.value).toBe(0);
        items.value = [];
        await nextTick();
        expect(cursor.index.value).toBe(null);
        expect(cursor.current.value).toBe(null);
    });

    it('composite key: same id across kinds tracks the right row', async () => {
        const { items, cursor } = harness([row('issue', 1), row('draft', 1)]);
        cursor.jump(2); // draft-1
        items.value = [row('draft', 1), row('issue', 1)];
        await nextTick();
        expect(cursor.current.value).toEqual(row('draft', 1));
        expect(cursor.index.value).toBe(0);
    });
});

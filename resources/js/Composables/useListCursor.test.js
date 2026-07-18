import { describe, it, expect, beforeEach } from 'vitest';
import { defineComponent, h, nextTick, ref } from 'vue';
import { mount } from '@vue/test-utils';
import { useListCursor } from './useListCursor';
import { registry } from './useAction';

// Drive the cursor inside a real component so useAction's mount/unmount lifecycle
// runs. `items` is a ref the test mutates to simulate re-sort/sync.
function harness(initial, options) {
    const items = ref(initial);
    let cursor;
    const Comp = defineComponent({
        setup() {
            cursor = useListCursor(() => items.value, (it) => it.kind + '-' + it.id, options);
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

    it('g g / G deselect and escape to the page edges', () => {
        const hits = [];
        const { cursor } = harness([row('issue', 1), row('issue', 2)], {
            onEscapeTop: () => hits.push('top'),
            onEscapeBottom: () => hits.push('bottom'),
        });
        cursor.move(1); // select something
        cursor.escapeBottom(); // G
        expect(cursor.index.value).toBe(null);
        cursor.move(1); // reselect
        cursor.escapeTop(); // g g
        expect(cursor.index.value).toBe(null);
        expect(hits).toEqual(['bottom', 'top']);
    });

    it('registers g g and Shift+G edge jumps', () => {
        harness([row('issue', 1)]);
        expect(registry.get('cursor:top').keys).toBe('g g');
        expect(registry.get('cursor:bottom').keys).toBe('Shift+G');
    });

    it('k past the top / j past the bottom deselect and call the escape hooks', () => {
        const hits = [];
        const { cursor } = harness([row('issue', 1), row('issue', 2)], {
            onEscapeTop: () => hits.push('top'),
            onEscapeBottom: () => hits.push('bottom'),
        });
        cursor.move(1); // → index 0
        cursor.move(-1); // k at top → escapeTop
        expect(cursor.index.value).toBe(null);
        cursor.move(1); // → index 0
        cursor.move(1); // → index 1 (last)
        cursor.move(1); // j at bottom → escapeBottom
        expect(cursor.index.value).toBe(null);
        expect(hits).toEqual(['top', 'bottom']);
    });

    it('does not wrap: j after escaping the bottom stays out; k re-enters at the last row', () => {
        const { cursor } = harness([row('issue', 1), row('issue', 2), row('issue', 3)], {
            onEscapeTop: () => {}, onEscapeBottom: () => {},
        });
        cursor.jump(3); // last row
        cursor.move(1); // j → escapeBottom (unset)
        expect(cursor.index.value).toBe(null);
        cursor.move(1); // j again → must NOT jump to top
        expect(cursor.index.value).toBe(null);
        cursor.move(-1); // k → re-enter at the last row
        expect(cursor.index.value).toBe(2);
    });

    it('does not wrap: k after escaping the top stays out; j re-enters at the first row', () => {
        const { cursor } = harness([row('issue', 1), row('issue', 2), row('issue', 3)], {
            onEscapeTop: () => {}, onEscapeBottom: () => {},
        });
        cursor.jump(1); // first row
        cursor.move(-1); // k → escapeTop (unset)
        cursor.move(-1); // k again → must NOT jump to bottom
        expect(cursor.index.value).toBe(null);
        cursor.move(1); // j → re-enter at the first row
        expect(cursor.index.value).toBe(0);
    });

    it('without escape hooks, j/k clamp at the ends (no deselect)', () => {
        const { cursor } = harness([row('issue', 1), row('issue', 2)]);
        cursor.move(1);
        cursor.move(-1);
        expect(cursor.index.value).toBe(0); // clamped at top, still selected
        cursor.move(1);
        cursor.move(1);
        expect(cursor.index.value).toBe(1); // clamped at bottom, still selected
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

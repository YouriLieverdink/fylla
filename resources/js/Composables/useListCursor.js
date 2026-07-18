import { computed, ref, watch } from 'vue';
import { useAction } from './useAction';

// Persistent j/k/digit row cursor over one primary list (#34, #42).
//
// - tracks by item key: follows the row across re-sort/sync; clamped-index
//   fallback if the tracked item leaves the list.
// - unset & invisible on load: first j/k lands on row 1; `current` is null while
//   unset, so a per-item verb closing over it is a no-op (mouse users never see it).
// - digits 1–9 jump to that visible row; 10+ reached via j/k (single-key, no timeout).
//
// The keys ride the layout's one guarded tinykeys listener via useAction, so the
// focus guard (#39) applies for free. j/k/1–9 are reserved app-wide — the
// `navigation` scope keeps them out of the page alphabet and out of the dynamic
// cheat-sheet (CheatSheet renders a static Navigation section instead).
export function useListCursor(items, keyOf = (item) => item.id) {
    const list = () => (typeof items === 'function' ? items() : items.value);

    const index = ref(null); // null = unset & invisible
    let trackedKey = null;

    function place(i) {
        index.value = i;
        trackedKey = keyOf(list()[i]);
    }

    function move(delta) {
        const n = list().length;
        if (n === 0) return;
        if (index.value === null) return place(0); // first j/k → row 1
        place(Math.min(Math.max(index.value + delta, 0), n - 1)); // clamp at ends
    }

    function jump(pos) {
        // 1-based positional jump; only lands on a row that exists.
        if (pos >= 1 && pos <= list().length) place(pos - 1);
    }

    // Re-anchor on every list change: keep the tracked item if it's still here,
    // else clamp the old index into the new bounds (the item left).
    watch(list, (next) => {
        if (index.value === null) return;
        const found = next.findIndex((it) => keyOf(it) === trackedKey);
        if (found !== -1) {
            index.value = found;
            return;
        }
        if (next.length === 0) {
            index.value = null;
            trackedKey = null;
            return;
        }
        place(Math.min(index.value, next.length - 1));
    });

    useAction({ id: 'cursor:down', label: 'Move cursor down', keys: 'j', scope: 'navigation', run: () => move(1) });
    useAction({ id: 'cursor:up', label: 'Move cursor up', keys: 'k', scope: 'navigation', run: () => move(-1) });
    for (let d = 1; d <= 9; d++) {
        useAction({ id: `cursor:jump-${d}`, label: `Jump to row ${d}`, keys: String(d), scope: 'navigation', run: () => jump(d) });
    }

    const activeKey = computed(() => {
        if (index.value === null) return null;
        const it = list()[index.value];
        return it ? keyOf(it) : null;
    });
    const current = computed(() => (index.value === null ? null : list()[index.value] ?? null));
    const isActive = (item) => activeKey.value !== null && keyOf(item) === activeKey.value;

    return { index, activeKey, current, isActive, move, jump };
}

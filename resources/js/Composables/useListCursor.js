import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { useAction } from './useAction';

// How many list cursors are live (0 on cursorless pages). The layout's global
// j/k scroll fallback (#42) is bound only while this is 0, so exactly one owner
// of j/k exists at a time — order-independent, unlike last-writer-wins.
export const activeCursorCount = ref(0);

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
export function useListCursor(items, keyOf = (item) => item.id, { onEscapeTop, onEscapeBottom } = {}) {
    const list = () => (typeof items === 'function' ? items() : items.value);

    const index = ref(null); // null = unset & invisible
    let trackedKey = null;
    // Which edge we last escaped past while unset: 'top' | 'bottom' | null (cold
    // start). Governs re-entry so j-past-the-bottom doesn't wrap around to the top.
    let escapedEdge = null;

    function place(i) {
        index.value = i;
        trackedKey = keyOf(list()[i]);
        escapedEdge = null;
    }

    // Deselect and escape past an end of the list (e.g. scroll to page top/bottom).
    function escapeTop() {
        index.value = null;
        trackedKey = null;
        escapedEdge = 'top';
        onEscapeTop?.();
    }
    function escapeBottom() {
        index.value = null;
        trackedKey = null;
        escapedEdge = 'bottom';
        onEscapeBottom?.();
    }

    function move(delta) {
        const n = list().length;
        if (n === 0) return;
        if (index.value === null) {
            // Re-enter from the edge we escaped past; only the first press from a
            // cold start jumps to the first target. No wrap-around.
            if (escapedEdge === 'bottom') return delta < 0 ? place(n - 1) : undefined;
            if (escapedEdge === 'top') return delta > 0 ? place(0) : undefined;
            return place(0);
        }
        if (delta < 0 && index.value === 0 && onEscapeTop) return escapeTop(); // k past the first target
        if (delta > 0 && index.value === n - 1 && onEscapeBottom) return escapeBottom(); // j past the last target
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

    onMounted(() => activeCursorCount.value++);
    onUnmounted(() => activeCursorCount.value--);

    useAction({ id: 'cursor:down', label: 'Move cursor down', keys: 'j', scope: 'navigation', run: () => move(1) });
    useAction({ id: 'cursor:up', label: 'Move cursor up', keys: 'k', scope: 'navigation', run: () => move(-1) });
    for (let d = 1; d <= 9; d++) {
        useAction({ id: `cursor:jump-${d}`, label: `Jump to row ${d}`, keys: String(d), scope: 'navigation', run: () => jump(d) });
    }
    useAction({ id: 'cursor:top', label: 'Jump to top', keys: 'g g', scope: 'navigation', run: escapeTop });
    useAction({ id: 'cursor:bottom', label: 'Jump to bottom', keys: 'Shift+G', scope: 'navigation', run: escapeBottom });

    const activeKey = computed(() => {
        if (index.value === null) return null;
        const it = list()[index.value];
        return it ? keyOf(it) : null;
    });
    const current = computed(() => (index.value === null ? null : list()[index.value] ?? null));
    const isActive = (item) => activeKey.value !== null && keyOf(item) === activeKey.value;

    return { index, activeKey, current, isActive, move, jump, escapeTop, escapeBottom };
}

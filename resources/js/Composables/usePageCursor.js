import { nextTick, watch } from 'vue';
import { useListCursor } from './useListCursor';

// useListCursor wired for a whole-page list (#43): escaping past either end
// scrolls the page to that edge, and the focused target is kept on screen. Every
// table/card page shares this so j/k navigates identically to the Worklist.
//
// Targets are usually plain string keys (identity keyOf) — highlight-only pages
// need no per-row object. Pass a keyOf when the targets are objects (Worklist).
export function usePageCursor(items, keyOf = (t) => t) {
    const cursor = useListCursor(items, keyOf, {
        onEscapeTop: () => window.scrollTo({ top: 0, behavior: 'smooth' }),
        onEscapeBottom: () => window.scrollTo({ top: document.body.scrollHeight, behavior: 'smooth' }),
    });

    // Keep the focused target on screen. `block: 'nearest'` only scrolls when the
    // target is actually off-screen, so a visible cursor never jumps the page.
    watch(() => cursor.activeKey.value, (key) => {
        if (key == null) return;
        nextTick(() => {
            document.querySelector(`[data-row="${CSS.escape(key)}"]`)?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
        });
    });

    return cursor;
}

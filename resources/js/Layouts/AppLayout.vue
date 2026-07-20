<script setup>
import { onMounted, onUnmounted, watch } from 'vue';
import { router } from '@inertiajs/vue3';
import { tinykeys, defaultKeybindingsHandlerIgnore } from 'tinykeys';
import { useAction, registerAction, unregisterAction, registry } from '../Composables/useAction';
import { activeCursorCount } from '../Composables/useListCursor';
import { openModalCount } from '../Composables/useModalGuard';
import CheatSheet from '../Components/CheatSheet.vue';

// Persistent Inertia layout (assigned via page.default.layout in app.js's
// resolve): this instance survives in-app navigation and partial reloads, so
// the one keybinding listener below is mounted once and lives for the session.

// Global workflow-loop actions (#33) declared here in the persistent layout.
// `.` = Sync now now flows through the registry — no direct binding left.
useAction({
    id: 'sync-now',
    label: 'Sync now',
    keys: '.',
    scope: 'global',
    // No preventDefault: a bare '.' has no default to suppress. The focus guard
    // already stops it from firing while typing periods in an input.
    run: () => router.post('/sync', {}, { preserveScroll: true }),
});

// Global g-leader navigation (#40; grammar #29, map #35). Depth-2 leader
// sequences dispatched natively by tinykeys — no timeout logic of our own.
// `g c` = Capacity (#29).
const NAV = [
    { keys: 'g w', href: '/',            label: 'Worklist' },
    { keys: 'g u', href: '/utilization', label: 'Utilization' },
    { keys: 'g c', href: '/capacity',    label: 'Capacity' },
    { keys: 'g e', href: '/estimation',  label: 'Estimation' },
    { keys: 'g d', href: '/delivery',    label: 'Delivery' },
    { keys: 'g s', href: '/settings',    label: 'Settings' },
];
for (const { keys, href, label } of NAV) {
    useAction({
        id: `nav:${href}`,
        label: `Go to ${label}`,
        keys,
        scope: 'global',
        run: () => router.visit(href),
    });
}

// Cursorless-page navigation fallback (#42): j/k scroll the viewport, g g / G
// jump to page top / bottom. Bound only while no list cursor is live
// (activeCursorCount === 0); on a page with a cursor those keys drive the cursor
// instead. Same `navigation` scope → the CheatSheet's static Navigation section
// covers both, no dynamic-group clutter.
const SCROLL_STEP = 80;
const smoothTo = (top) => window.scrollTo({ top, behavior: 'smooth' });
const FALLBACK = [
    { id: 'scroll-down', label: 'Scroll / cursor down', keys: 'j', scope: 'navigation', run: () => window.scrollBy({ top: SCROLL_STEP, behavior: 'smooth' }) },
    { id: 'scroll-up', label: 'Scroll / cursor up', keys: 'k', scope: 'navigation', run: () => window.scrollBy({ top: -SCROLL_STEP, behavior: 'smooth' }) },
    { id: 'jump-top', label: 'Jump to top', keys: 'g g', scope: 'navigation', run: () => smoothTo(0) },
    { id: 'jump-bottom', label: 'Jump to bottom', keys: 'Shift+G', scope: 'navigation', run: () => smoothTo(document.body.scrollHeight) },
];
watch(activeCursorCount, (n) => {
    for (const a of FALLBACK) n > 0 ? unregisterAction(a.id) : registerAction(a);
}, { immediate: true });

// Focus guard (#39, spec #30): Escape is the sole exception, so a bound Escape
// still fires in any context. `data-kb-ignore` is the opt-out hatch for custom
// popovers (e.g. CellEditor) that aren't natively editable — ancestor-aware.
// Everything else defers to tinykeys' own ignore (editable contexts +
// repeat/isComposing). Native Tab/Shift-Tab flow is untouched: nothing binds them.
function ignore(event) {
    // Modal guard (#43): while a blocking modal is open, suppress every registry
    // binding — page-local, j/k cursor, and global (g-nav / . / ?) alike, and
    // Escape too. The open modal exits via its own native @keydown.esc handler,
    // not the registry, so nothing survives beneath the scrim.
    if (openModalCount.value > 0) return true;
    if (event.key === 'Escape') return false;
    if (event.target instanceof Element && event.target.closest('[data-kb-ignore]')) return true;
    // j/k (cursor move / page scroll) fire on key-repeat too, so holding continues
    // — the default ignore drops event.repeat. Still suppressed in editable contexts.
    if (event.key === 'j' || event.key === 'k') {
        const t = event.target;
        return event.isComposing || (t !== event.currentTarget && t instanceof Element && t.matches('[contenteditable],input,select,textarea'));
    }
    return defaultKeybindingsHandlerIgnore(event);
}

let unsubscribe;

// Rebuild the tinykeys keymap from the live registry. Two tinykeys quirks bite a
// single long-lived listener that mixes `g`-leader sequences with the bare page
// keys they collide on (e.g. `g c` nav vs `c` capture) — both are handled here.
const isSequence = (action) => action.keys.includes(' '); // multi-press leader binding, e.g. `g c`
function bind() {
    unsubscribe?.();
    const keymap = {};
    // (1) Iteration order. tinykeys walks the keymap in insertion order and, on the
    // first complete match, fires + `break`s. Child pages mount before this layout,
    // so their bare keys register first — a bare `c` would then win over `g c` on
    // the second keypress. Register multi-key sequences first so a leader sequence
    // always beats a bare key that shares its final letter, whatever the mount order.
    const actions = [...registry.values()].sort((a, b) => isSequence(b) - isSequence(a));
    for (const action of actions) {
        // (2) Stale progress. On a completed sequence tinykeys `break`s without
        // clearing the sibling sequences it armed on the leader press; those linger
        // until the 1s timeout and disarm the next leader press within that window
        // (so a second `g`-nav soon after falls through to a bare key). Rebind after
        // a sequence fires to drop them. Single keys arm nothing, so they skip it —
        // hold-to-repeat j/k stays churn-free.
        keymap[action.keys] = isSequence(action)
            ? (event) => { action.run(event); bind(); }
            : action.run;
    }
    unsubscribe = tinykeys(window, keymap, { ignore });
}

onMounted(() => {
    bind();
    // Rebind whenever the live action set changes — any component mounting or
    // unmounting a useAction() adds/removes a keystroke.
    watch(registry, bind);
});

onUnmounted(() => unsubscribe?.());
</script>

<template>
    <slot />
    <CheatSheet />
</template>

<script setup>
import { onMounted, onUnmounted, watch } from 'vue';
import { router } from '@inertiajs/vue3';
import { tinykeys, defaultKeybindingsHandlerIgnore } from 'tinykeys';
import { useAction, registry } from '../Composables/useAction';
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
// `g c` = Capacity, `g l` = cLients: mnemonic split of the c-collision (#29).
const NAV = [
    { keys: 'g w', href: '/',            label: 'Worklist' },
    { keys: 'g u', href: '/utilization', label: 'Utilization' },
    { keys: 'g c', href: '/capacity',    label: 'Capacity' },
    { keys: 'g e', href: '/estimation',  label: 'Estimation' },
    { keys: 'g l', href: '/clients',     label: 'Clients' },
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

// Focus guard (#39, spec #30): Escape is the sole exception, so a bound Escape
// still fires in any context. `data-kb-ignore` is the opt-out hatch for custom
// popovers (e.g. CellEditor) that aren't natively editable — ancestor-aware.
// Everything else defers to tinykeys' own ignore (editable contexts +
// repeat/isComposing). Native Tab/Shift-Tab flow is untouched: nothing binds them.
function ignore(event) {
    if (event.key === 'Escape') return false;
    if (event.target instanceof Element && event.target.closest('[data-kb-ignore]')) return true;
    return defaultKeybindingsHandlerIgnore(event);
}

let unsubscribe;

// Rebuild the tinykeys keymap from the live registry.
function bind() {
    unsubscribe?.();
    const keymap = {};
    for (const action of registry.values()) {
        keymap[action.keys] = action.run;
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

<script setup>
import { onMounted, onUnmounted, watch } from 'vue';
import { router } from '@inertiajs/vue3';
import { tinykeys, defaultKeybindingsHandlerIgnore } from 'tinykeys';
import { useAction, registry } from '../Composables/useAction';

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
</template>

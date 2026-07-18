<script setup>
import { onMounted, onUnmounted, watch } from 'vue';
import { router } from '@inertiajs/vue3';
import { tinykeys } from 'tinykeys';
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
    // No preventDefault: a bare '.' has no default to suppress, and eating it
    // would block typing periods in inputs before the focus guard exists.
    run: () => router.post('/sync', {}, { preserveScroll: true }),
});

let unsubscribe;

// Rebuild the tinykeys keymap from the live registry.
function bind() {
    unsubscribe?.();
    const keymap = {};
    for (const action of registry.values()) {
        keymap[action.keys] = action.run;
    }
    unsubscribe = tinykeys(window, keymap);
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

<script setup>
import { onMounted, onUnmounted } from 'vue';
import { router } from '@inertiajs/vue3';
import { tinykeys } from 'tinykeys';

// Persistent Inertia layout (assigned via page.default.layout in app.js's
// resolve): this component instance survives in-app navigation and partial
// reloads, so the one keybinding listener below is mounted once and lives for
// the whole session.
let unsubscribe;

onMounted(() => {
    unsubscribe = tinykeys(window, {
        // Tracer binding (#37): prove the listener path end-to-end. Bound direct,
        // no registry and no focus guard yet — those land in the next slices.
        // No preventDefault: a bare '.' has no default to suppress, and eating it
        // would block typing periods in inputs before the focus guard exists.
        '.': () => router.post('/sync', {}, { preserveScroll: true }),
    });
});

onUnmounted(() => unsubscribe?.());
</script>

<template>
    <slot />
</template>

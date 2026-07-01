<script setup>
// Base surface card. radius/padding vary per use, so they're props rather than
// fixed classes; `accent` swaps to the periwinkle-tinted elevation.
defineProps({
    radius: { type: String, default: '20px' },
    pad: { type: String, default: '28px 30px' },
    accent: { type: Boolean, default: false },
});
</script>

<template>
    <div
        class="border border-card-border bg-surface"
        :class="accent ? 'shadow-card-accent' : 'shadow-card'"
        :style="{ borderRadius: radius, padding: pad }"
    >
        <slot />
    </div>
</template>

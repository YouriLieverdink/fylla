<script setup>
// Nested surface — a step quieter than the base card.
defineProps({
    title: { type: String, default: '' },
});
</script>

<template>
    <div class="rounded-2xl border border-border-soft bg-surface-soft px-[18px] py-4">
        <div v-if="title" class="mb-[5px] text-[13px] font-semibold">{{ title }}</div>
        <slot />
    </div>
</template>

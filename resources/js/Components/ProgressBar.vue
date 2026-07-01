<script setup>
defineProps({
    value: { type: Number, default: 0 }, // percent 0–100
    tone: { type: String, default: 'track' }, // track | behind | accent
    height: { type: String, default: '8px' },
});

const fill = { track: 'bg-track', behind: 'bg-behind', accent: 'bg-accent' };
</script>

<template>
    <div class="overflow-hidden rounded-full bg-sunken" :style="{ height }">
        <div
            class="h-full rounded-full"
            :class="fill[tone]"
            :style="{ width: Math.min(value, 100) + '%' }"
        ></div>
    </div>
</template>

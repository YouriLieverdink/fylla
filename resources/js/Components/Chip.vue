<script setup>
defineProps({
    tone: { type: String, default: 'neutral' }, // billable | accent | neutral | track | behind
    dot: { type: Boolean, default: false },
});

const tones = {
    billable: 'text-accent-deep bg-accent-tint',
    accent: 'text-accent-deep bg-accent-tint',
    neutral: 'text-[#8a8578] bg-divider',
    track: 'text-track bg-track-tint',
    behind: 'text-behind bg-behind-tint',
};

const dotColor = {
    billable: 'bg-accent',
    accent: 'bg-accent',
    neutral: 'bg-faint-3',
    track: 'bg-track',
    behind: 'bg-behind',
};
</script>

<template>
    <span
        :class="[
            'inline-flex items-center gap-1.5 rounded-full px-3 py-[7px] font-mono text-[12px] font-medium',
            tones[tone],
        ]"
    >
        <span v-if="dot" :class="['inline-block h-1.5 w-1.5 rounded-full', dotColor[tone]]"></span>
        <slot />
    </span>
</template>

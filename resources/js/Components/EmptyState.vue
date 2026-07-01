<script setup>
import Card from './Card.vue';

defineProps({
    title: { type: String, default: '' },
    text: { type: String, default: '' },
});
</script>

<template>
    <Card pad="40px 28px" class="text-center">
        <div
            class="mx-auto mb-4 flex h-[52px] w-[52px] items-center justify-center rounded-2xl border border-border-soft bg-canvas"
        >
            <slot name="icon">
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
                    <circle cx="12" cy="12" r="8.5" stroke="var(--color-faint-4)" stroke-width="1.5" />
                    <path
                        d="M12 8v4l2.5 1.5"
                        stroke="var(--color-faint-4)"
                        stroke-width="1.5"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                    />
                </svg>
            </slot>
        </div>
        <div class="mb-1.5 text-[15px] font-semibold">{{ title }}</div>
        <div class="mx-auto mb-5 max-w-[36ch] text-[13px] leading-[1.55] text-faint-2">{{ text }}</div>
        <slot name="action" />
    </Card>
</template>

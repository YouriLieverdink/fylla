<script setup>
defineProps({
    variant: { type: String, default: 'primary' }, // primary | secondary | ghost | disabled
    size: { type: String, default: 'md' }, // md | sm
    dot: { type: Boolean, default: false },
});

const variants = {
    primary: 'border-0 bg-accent text-white shadow-btn hover:opacity-90',
    secondary: 'border border-[#e0dbd0] bg-surface text-ink-soft hover:bg-canvas',
    ghost: 'border-0 bg-transparent text-muted hover:bg-canvas',
    disabled: 'border-0 bg-divider text-faint-3 cursor-not-allowed',
};

const sizes = {
    md: 'text-[14px] px-5 py-3 rounded-[13px]',
    sm: 'text-[13px] px-[14px] py-[10px] rounded-[11px]',
};
</script>

<template>
    <button
        :disabled="variant === 'disabled'"
        :class="[
            'inline-flex cursor-pointer items-center justify-center gap-2 font-sans font-semibold transition',
            variants[variant],
            sizes[size],
        ]"
    >
        <span v-if="dot" class="h-1.5 w-1.5 rounded-full bg-accent"></span>
        <slot />
    </button>
</template>

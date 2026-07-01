<script setup>
defineProps({
    options: { type: Array, default: () => [] },
    modelValue: { type: String, default: '' },
});
defineEmits(['update:modelValue']);
</script>

<template>
    <div class="inline-flex gap-0.5 rounded-[14px] bg-sunken p-1">
        <button
            v-for="opt in options"
            :key="opt"
            class="cursor-pointer rounded-[11px] px-5 py-2.5 font-sans text-[13.5px] font-semibold transition"
            :class="
                opt === modelValue
                    ? 'bg-surface text-ink shadow-[0_2px_6px_-2px_rgba(42,41,38,0.14)]'
                    : 'bg-transparent text-[#8a8578]'
            "
            @click="$emit('update:modelValue', opt)"
        >
            {{ opt }}
        </button>
    </div>
</template>

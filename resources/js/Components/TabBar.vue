<script setup>
defineProps({
    tabs: { type: Array, default: () => [] },
    modelValue: { type: String, default: '' },
});
defineEmits(['update:modelValue']);
</script>

<template>
    <div class="flex gap-6 border-b border-border-soft">
        <button
            v-for="tab in tabs"
            :key="tab"
            class="cursor-pointer pb-3 font-sans text-[13.5px] transition"
            :class="
                tab === modelValue
                    ? 'border-b-2 border-accent font-semibold text-ink'
                    : 'font-medium text-faint'
            "
            @click="$emit('update:modelValue', tab)"
        >
            {{ tab }}
        </button>
    </div>
</template>

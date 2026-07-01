<script setup>
import Card from './Card.vue';
import AppButton from './AppButton.vue';

defineProps({
    label: { type: String, default: 'Synced with issue tracker' },
    lastSynced: { type: String, default: '' },
});
defineEmits(['sync']);
</script>

<template>
    <Card pad="20px 24px" class="flex items-center justify-between gap-6">
        <div class="flex items-center gap-3">
            <span class="h-2 w-2 rounded-full bg-track shadow-[0_0_0_4px_var(--color-track-tint)]"></span>
            <div>
                <div class="font-sans text-[13px] font-medium text-ink-soft">{{ label }}</div>
                <div class="mt-[5px] font-mono text-[11.5px] font-medium text-faint-2">{{ lastSynced }}</div>
            </div>
        </div>
        <AppButton variant="secondary" size="sm" @click="$emit('sync')">
            <svg width="13" height="13" viewBox="0 0 14 14" fill="none">
                <path
                    d="M12 7a5 5 0 1 1-1.46-3.54M12 2v3h-3"
                    stroke="currentColor"
                    stroke-width="1.5"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                />
            </svg>
            Sync now
        </AppButton>
    </Card>
</template>

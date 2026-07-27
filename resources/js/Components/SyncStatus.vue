<script setup>
import AppButton from './AppButton.vue';

defineProps({
    label: { type: String, default: 'Synced with issue tracker' },
    lastSynced: { type: String, default: '' },
    syncing: { type: Boolean, default: false },
    // Dispatched, but no job has started yet — the gap the queue introduces.
    queued: { type: Boolean, default: false },
});
defineEmits(['sync']);
</script>

<template>
    <div class="flex items-center gap-[14px]">
        <div class="text-right">
            <div class="font-sans text-[12.5px] font-medium text-ink-soft">{{ label }}</div>
            <div class="mt-1 font-mono text-[11px] font-medium text-faint-2">{{ lastSynced }}</div>
        </div>
        <span class="h-2 w-2 rounded-full bg-track shadow-[0_0_0_4px_var(--color-track-tint)]"></span>
        <AppButton :variant="syncing || queued ? 'disabled' : 'secondary'" size="sm" @click="$emit('sync')">
            <!-- Queued: the dispatch landed but no worker has picked it up yet. A
                 tick rather than a spinner, so "accepted" reads apart from "working". -->
            <svg v-if="queued" width="13" height="13" viewBox="0 0 14 14" fill="none">
                <path
                    d="M2.5 7.5 5.5 10.5 11.5 3.5"
                    stroke="currentColor"
                    stroke-width="1.5"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                />
            </svg>
            <svg v-else width="13" height="13" viewBox="0 0 14 14" fill="none" :class="{ 'animate-spin': syncing }">
                <path
                    d="M12 7a5 5 0 1 1-1.46-3.54M12 2v3h-3"
                    stroke="currentColor"
                    stroke-width="1.5"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                />
            </svg>
            {{ queued ? 'Queued' : syncing ? 'Syncing…' : 'Sync now' }}
        </AppButton>
    </div>
</template>

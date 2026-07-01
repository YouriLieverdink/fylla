<script setup>
import { router } from '@inertiajs/vue3';
import Card from '../Components/Card.vue';
import SyncStatus from '../Components/SyncStatus.vue';
import EmptyState from '../Components/EmptyState.vue';
import AppButton from '../Components/AppButton.vue';

defineProps({
    issues: { type: Array, default: () => [] },
    lastSyncedAt: { type: String, default: null },
});

function syncNow() {
    router.post('/sync', {}, { preserveScroll: true });
}

function fmt(ts) {
    return ts ? new Date(ts).toLocaleString() : '—';
}

// type → the coloured square from the kit's work-item rows
const typeDot = { Feature: 'bg-accent-soft', Bug: 'bg-behind', Task: 'bg-faint-2' };

const cols = 'grid-cols-[80px_1fr_120px_170px]';
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-[60px]">
        <!-- header -->
        <header class="mb-8 flex items-center justify-between gap-6">
            <div>
                <div class="flex items-center gap-3">
                    <div class="relative h-[34px] w-[34px] rounded-[11px] bg-accent shadow-[0_5px_15px_-5px_rgba(108,95,201,0.6)]">
                        <div class="absolute inset-0 flex items-center justify-center">
                            <div
                                class="h-3 w-3 rounded-full border-[2.5px] border-white border-t-transparent"
                                style="transform: rotate(35deg)"
                            ></div>
                        </div>
                    </div>
                    <span class="text-[22px] font-semibold tracking-[-0.02em]">Fylla</span>
                </div>
                <p class="mt-3 text-[13px] text-muted">
                    {{ issues.length }} {{ issues.length === 1 ? 'issue' : 'issues' }} synced from the tracker.
                </p>
            </div>
            <SyncStatus
                label="Synced with Kendo"
                :last-synced="lastSyncedAt ? 'last synced ' + fmt(lastSyncedAt) : 'never synced'"
                @sync="syncNow"
            />
        </header>

        <!-- issues -->
        <Card v-if="issues.length" radius="24px" pad="10px 10px 12px">
            <div
                class="grid gap-3 px-5 py-2 font-mono text-[10px] font-semibold uppercase tracking-[0.1em] text-faint-3"
                :class="cols"
            >
                <span>Key</span>
                <span>Title</span>
                <span>Priority</span>
                <span class="text-right">Updated</span>
            </div>

            <div class="flex flex-col">
                <div
                    v-for="issue in issues"
                    :key="issue.key"
                    class="grid items-center gap-3 rounded-[14px] border-t border-divider-soft px-5 py-3.5 transition hover:bg-surface-soft"
                    :class="cols"
                >
                    <span class="font-mono text-[12px] font-semibold text-muted">{{ issue.key }}</span>
                    <div class="min-w-0">
                        <div class="flex items-center gap-2">
                            <span
                                class="h-[7px] w-[7px] flex-none rounded-sm"
                                :class="typeDot[issue.type] ?? 'bg-faint-2'"
                                :title="issue.type"
                            ></span>
                            <span class="truncate text-[14px] font-medium">{{ issue.title }}</span>
                        </div>
                        <div v-if="issue.type" class="mt-[3px] font-mono text-[11px] text-faint-3">{{ issue.type }}</div>
                    </div>
                    <div>
                        <span
                            v-if="issue.priority"
                            class="rounded-[7px] bg-divider px-[9px] py-[5px] font-mono text-[11px] font-medium text-[#8a8578]"
                            >{{ issue.priority }}</span
                        >
                        <span v-else class="font-mono text-[11px] text-faint-3">—</span>
                    </div>
                    <div class="text-right font-mono text-[12px] tabular-nums text-faint">{{ fmt(issue.updated_at) }}</div>
                </div>
            </div>
        </Card>

        <EmptyState
            v-else
            title="No issues yet"
            text="Nothing has synced from Kendo yet. Pull your assigned issues to get started."
        >
            <template #action>
                <AppButton variant="primary" size="sm" @click="syncNow">Sync now</AppButton>
            </template>
        </EmptyState>
    </div>
</template>

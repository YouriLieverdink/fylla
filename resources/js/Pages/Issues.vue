<script setup>
import { router, usePoll } from '@inertiajs/vue3';
import Card from '../Components/Card.vue';
import AppHeader from '../Components/AppHeader.vue';
import Chip from '../Components/Chip.vue';
import EmptyState from '../Components/EmptyState.vue';
import AppButton from '../Components/AppButton.vue';
import BillableMetric from '../Components/BillableMetric.vue';
import UtilizationTrendChart from '../Components/UtilizationTrendChart.vue';
import TimerStack from '../Components/TimerStack.vue';

const props = defineProps({
    issues: { type: Array, default: () => [] },
    timer: { type: Object, default: null },
    liveIssueIds: { type: Array, default: () => [] },
    utilization: { type: Object, default: () => ({}) },
});

const opts = { preserveScroll: true };

// keep issues fresh when the 15-min scheduled sync fires; narrow only: leaves
// the running timer clock untouched (ticks locally off started_at)
usePoll(60000, { only: ['issues', 'utilization'] });

function syncNow() {
    router.post('/sync', {}, opts);
}

function startTimer(issue) {
    router.post('/timers', { issue_id: issue.id }, opts);
}

// minutes → "6h" / "1.5h"; em-dash when unset
function hrs(min) {
    if (min == null) return '—';
    const h = min / 60;
    return (Number.isInteger(h) ? h : h.toFixed(1)) + 'h';
}

// type → the coloured square from the kit's work-item rows
const typeDot = { Feature: 'bg-accent-soft', Bug: 'bg-behind', Task: 'bg-faint-2' };

const cols = 'grid-cols-[66px_1fr_78px_90px_74px_96px]';
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-11">
        <!-- header -->
        <AppHeader />

        <!-- metrics row -->
        <div class="mb-[22px] grid items-stretch gap-[22px] lg:grid-cols-[400px_1fr]">
            <BillableMetric
                :value="utilization.value"
                :status="utilization.status"
                :delta="utilization.delta"
                :delta-caption="utilization.deltaCaption"
                :target="utilization.target"
                :note="utilization.note"
                :week="utilization.week"
            />
            <UtilizationTrendChart :points="utilization.points" :target="utilization.target" />
        </div>

        <!-- timer stack -->
        <div class="mb-[22px]">
            <TimerStack
                :active="timer?.active ?? null"
                :paused="timer?.paused ?? []"
                @pause="router.post('/timers/pause', {}, opts)"
                @resume="router.post('/timers/resume', {}, opts)"
                @stop="router.post('/timers/stop', {}, opts)"
                @note="(text) => router.post('/timers/notes', { text }, opts)"
            />
        </div>

        <!-- work items -->
        <Card v-if="issues.length" radius="24px" pad="10px 10px 12px">
            <div class="flex items-center justify-between px-5 pb-3.5 pt-4">
                <div class="text-[16px] font-semibold tracking-[-0.01em]">Work items</div>
                <Chip tone="accent">In progress · {{ liveIssueIds.length }}</Chip>
            </div>

            <div
                class="grid gap-3 px-5 py-2 font-mono text-[10px] font-semibold uppercase tracking-[0.1em] text-faint-3"
                :class="cols"
            >
                <span>Key</span>
                <span>Title</span>
                <span class="text-right">Estimate</span>
                <span class="text-right">Remaining</span>
                <span class="text-right">Priority</span>
                <span></span>
            </div>

            <div class="flex flex-col">
                <div
                    v-for="issue in issues"
                    :key="issue.key"
                    class="grid items-center gap-3 rounded-[14px] border-t border-divider-soft px-5 py-3.5 transition"
                    :class="[cols, liveIssueIds.includes(issue.id) ? 'bg-surface-soft' : 'hover:bg-surface-soft']"
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
                    <div class="text-right font-mono text-[13px] font-medium tabular-nums text-muted">
                        {{ hrs(issue.estimated_minutes) }}
                    </div>
                    <div
                        class="text-right font-mono text-[13px] font-medium tabular-nums"
                        :class="
                            issue.remaining_minutes == null
                                ? 'text-faint-3'
                                : issue.estimated_minutes != null && issue.remaining_minutes >= issue.estimated_minutes
                                  ? 'text-behind'
                                  : 'text-track'
                        "
                    >
                        {{ hrs(issue.remaining_minutes) }}
                    </div>
                    <div class="text-right">
                        <span
                            v-if="issue.priority"
                            class="rounded-[7px] bg-divider px-[9px] py-[5px] font-mono text-[11px] font-medium text-[#8a8578]"
                            >{{ issue.priority }}</span
                        >
                        <span v-else class="font-mono text-[11px] text-faint-3">—</span>
                    </div>
                    <div class="flex justify-end">
                        <span
                            v-if="liveIssueIds.includes(issue.id)"
                            class="inline-flex items-center gap-1.5 rounded-[10px] bg-accent-tint px-3 py-2 font-mono text-[11px] font-semibold uppercase tracking-[0.06em] text-accent-deep"
                        >
                            <span class="h-1.5 w-1.5 rounded-full bg-accent" style="animation: fyl-pulse 2s ease-in-out infinite"></span>
                            live
                        </span>
                        <button
                            v-else
                            class="inline-flex cursor-pointer items-center gap-[7px] rounded-[10px] border border-[#e0dbd0] bg-white px-[13px] py-2 font-sans text-[12.5px] font-semibold text-ink-soft transition hover:border-accent-tint-2 hover:bg-[#faf9fd]"
                            @click="startTimer(issue)"
                        >
                            <span class="h-1.5 w-1.5 rounded-full bg-accent"></span>
                            Start
                        </button>
                    </div>
                </div>
            </div>
        </Card>

        <EmptyState
            v-else
            title="No work items synced"
            text="Nothing assigned to you right now. Pull the latest from your issue tracker to populate this view."
        >
            <template #action>
                <AppButton variant="primary" size="sm" @click="syncNow">Sync now</AppButton>
            </template>
        </EmptyState>
    </div>
</template>

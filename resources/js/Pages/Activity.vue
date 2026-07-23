<script setup>
import AppHeader from '../Components/AppHeader.vue';
import Card from '../Components/Card.vue';
import EmptyState from '../Components/EmptyState.vue';

const props = defineProps({
    runs: { type: Array, default: () => [] },
});

const triggerLabel = { scheduled: 'Scheduled', manual: 'Sync now', 'worklog-post': 'Worklog post' };

function fmt(ts) {
    return ts ? new Date(ts).toLocaleString() : '—';
}

// Wall-clock run time; blank while running (no finished_at yet).
function dur(run) {
    if (!run.finishedAt) return '';
    const ms = new Date(run.finishedAt) - new Date(run.startedAt);
    return ms >= 1000 ? (ms / 1000).toFixed(1) + 's' : ms + 'ms';
}
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-11">
        <AppHeader />

        <div class="mb-8">
            <h1 class="mb-3 text-[34px] font-bold leading-[1.05] tracking-[-0.03em]">Activity</h1>
            <p class="max-w-[62ch] text-[15px] leading-[1.55] text-muted">
                Every background job run — scheduled syncs, manual "Sync now", and worklog posts —
                newest first.
            </p>
        </div>

        <Card v-if="runs.length" pad="8px 0">
            <table class="w-full border-collapse text-[13px]">
                <thead>
                    <tr class="border-b border-divider-soft text-left font-mono text-[11px] uppercase tracking-[0.08em] text-faint-3">
                        <th class="px-6 py-3 font-medium">Status</th>
                        <th class="px-6 py-3 font-medium">Trigger</th>
                        <th class="px-6 py-3 font-medium">Job</th>
                        <th class="px-6 py-3 font-medium">Started</th>
                        <th class="px-6 py-3 text-right font-medium">Duration</th>
                    </tr>
                </thead>
                <tbody>
                    <tr
                        v-for="run in runs"
                        :key="run.id"
                        class="border-b border-divider-soft align-top last:border-0"
                        :class="run.status === 'failed' && 'bg-behind/[0.03]'"
                    >
                        <td class="whitespace-nowrap px-6 py-3">
                            <span class="inline-flex items-center gap-2">
                                <svg
                                    v-if="run.status === 'running'"
                                    width="12"
                                    height="12"
                                    viewBox="0 0 14 14"
                                    fill="none"
                                    class="animate-spin text-accent"
                                >
                                    <path d="M12 7a5 5 0 1 1-1.46-3.54M12 2v3h-3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
                                </svg>
                                <span
                                    v-else
                                    class="h-2 w-2 rounded-full"
                                    :class="run.status === 'failed' ? 'bg-behind' : 'bg-track'"
                                ></span>
                                <span class="font-mono text-[11px] uppercase tracking-[0.08em] text-faint-2">{{ run.status }}</span>
                            </span>
                        </td>
                        <td class="whitespace-nowrap px-6 py-3 text-muted">{{ triggerLabel[run.trigger] || run.trigger }}</td>
                        <td class="px-6 py-3">
                            <span class="font-medium">{{ run.jobClass }}</span>
                            <div v-if="run.error" class="mt-1 max-w-[520px] truncate font-mono text-[11px] text-behind">{{ run.error }}</div>
                        </td>
                        <td class="whitespace-nowrap px-6 py-3 text-faint-2">{{ fmt(run.startedAt) }}</td>
                        <td class="whitespace-nowrap px-6 py-3 text-right tabular-nums text-muted">{{ dur(run) }}</td>
                    </tr>
                </tbody>
            </table>
        </Card>

        <EmptyState
            v-else
            title="No activity yet"
            text="Background job runs land here as they happen. Trigger a sync or wait for the scheduled one."
        />
    </div>
</template>

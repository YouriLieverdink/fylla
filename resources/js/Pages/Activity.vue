<script setup>
import { computed, ref } from 'vue';
import AppHeader from '../Components/AppHeader.vue';
import Card from '../Components/Card.vue';
import EmptyState from '../Components/EmptyState.vue';

const props = defineProps({
    // Runs grouped by sync moment (#88); each moment carries its rolled-up
    // status/failedCount and its constituent runs. Newest moment first.
    moments: { type: Array, default: () => [] },
});

const triggerLabel = { scheduled: 'Scheduled sync', manual: 'Sync now', 'worklog-post': 'Worklog post' };
const triggerDot = { scheduled: 'bg-faint-2', manual: 'bg-accent', 'worklog-post': 'bg-track' };

const failureCount = computed(() => props.moments.reduce((n, m) => n + m.failedCount, 0));

function fmt(ts) {
    return ts ? new Date(ts).toLocaleString() : '—';
}

// Wall-clock run time; blank while running (no finished_at yet).
function dur(run) {
    if (!run.finishedAt) return '';
    const ms = new Date(run.finishedAt) - new Date(run.startedAt);
    return ms >= 1000 ? (ms / 1000).toFixed(1) + 's' : ms + 'ms';
}

// Pre-expand the moments that want attention: anything failed or still running.
const expanded = ref(
    new Set(props.moments.filter((m) => m.status !== 'ok').map((m) => m.id)),
);
function toggle(id) {
    const s = new Set(expanded.value);
    s.has(id) ? s.delete(id) : s.add(id);
    expanded.value = s;
}
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-11">
        <AppHeader />

        <div class="mb-8 flex items-end justify-between gap-6">
            <div>
                <h1 class="mb-3 text-[34px] font-bold leading-[1.05] tracking-[-0.03em]">Activity</h1>
                <p class="max-w-[62ch] text-[15px] leading-[1.55] text-muted">
                    Every background job run — grouped by sync moment. Expand a fan-out to see its
                    jobs; worklog posts stand alone. Newest first.
                </p>
            </div>
            <div
                v-if="failureCount"
                class="flex flex-none items-center gap-2 rounded-full bg-behind/10 px-3 py-1.5"
            >
                <span class="h-2 w-2 rounded-full bg-behind"></span>
                <span class="font-mono text-[11px] font-semibold text-behind">{{ failureCount }} failed</span>
            </div>
        </div>

        <div v-if="moments.length" class="flex flex-col gap-3">
            <Card v-for="m in moments" :key="m.id" pad="0" :accent="m.status === 'running'">
                <button
                    type="button"
                    class="flex w-full items-center gap-4 px-6 py-4 text-left"
                    @click="toggle(m.id)"
                >
                    <svg
                        v-if="m.status === 'running'"
                        width="15"
                        height="15"
                        viewBox="0 0 14 14"
                        fill="none"
                        class="flex-none animate-spin text-accent"
                    >
                        <path d="M12 7a5 5 0 1 1-1.46-3.54M12 2v3h-3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
                    </svg>
                    <span
                        v-else
                        class="h-2.5 w-2.5 flex-none rounded-full"
                        :class="m.status === 'failed' ? 'bg-behind' : 'bg-track'"
                    ></span>
                    <span class="flex items-center gap-2">
                        <span class="h-1.5 w-1.5 rounded-full" :class="triggerDot[m.trigger]"></span>
                        <span class="text-[14px] font-semibold">{{ triggerLabel[m.trigger] || m.trigger }}</span>
                    </span>
                    <span class="font-mono text-[11px] text-faint-2">
                        {{ m.runs.length }} job{{ m.runs.length > 1 ? 's' : '' }}
                    </span>
                    <span v-if="m.status === 'failed'" class="font-mono text-[11px] font-semibold text-behind">
                        {{ m.failedCount }} failed
                    </span>
                    <span v-else-if="m.status === 'running'" class="font-mono text-[11px] font-semibold text-accent">running…</span>
                    <span class="flex-1"></span>
                    <span class="font-mono text-[11px] text-faint-2">{{ fmt(m.startedAt) }}</span>
                    <span
                        class="font-mono text-[11px] text-faint transition-transform"
                        :class="expanded.has(m.id) ? 'rotate-90' : ''"
                    >›</span>
                </button>
                <div v-if="expanded.has(m.id)" class="border-t border-divider-soft bg-canvas/40 px-6 py-2">
                    <div v-for="r in m.runs" :key="r.id" class="flex items-center gap-3 py-2">
                        <svg
                            v-if="r.status === 'running'"
                            width="12"
                            height="12"
                            viewBox="0 0 14 14"
                            fill="none"
                            class="flex-none animate-spin text-accent"
                        >
                            <path d="M12 7a5 5 0 1 1-1.46-3.54M12 2v3h-3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
                        </svg>
                        <span
                            v-else
                            class="h-1.5 w-1.5 flex-none rounded-full"
                            :class="r.status === 'failed' ? 'bg-behind' : 'bg-track'"
                        ></span>
                        <span class="flex-1 truncate text-[12.5px] font-medium">{{ r.jobClass }}</span>
                        <span v-if="r.error" class="max-w-[320px] truncate font-mono text-[10.5px] text-behind">{{ r.error }}</span>
                        <span v-else-if="r.status === 'running'" class="font-mono text-[10.5px] text-accent">running…</span>
                        <span v-else class="font-mono text-[10.5px] text-faint-2">{{ dur(r) }}</span>
                    </div>
                </div>
            </Card>
        </div>

        <EmptyState
            v-else
            title="No activity yet"
            text="Background job runs land here as they happen. Trigger a sync or wait for the scheduled one."
        />
    </div>
</template>

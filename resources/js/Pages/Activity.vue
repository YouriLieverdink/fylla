<script setup>
import { router } from '@inertiajs/vue3';
import { computed, ref, watch } from 'vue';
import { useActivityChannel } from '../Composables/useActivityChannel';
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

// Live view: runs go running → ok/failed while the page is open, pushed over
// the socket (#92). No poll floor — `online` renders the tell when it's down.
const { online } = useActivityChannel(['moments']);

// Auto-expand a moment the *first* time it's seen: the newest one always, plus
// anything wanting attention (failed or still running). Tracking what's been
// seen is what keeps a reload from re-opening a card the user just collapsed.
const expanded = ref(new Set());
const seen = new Set();

function absorb(moments) {
    const fresh = moments.filter((m) => !seen.has(m.id));
    fresh.forEach((m) => seen.add(m.id));
    const open = fresh.filter((m) => m.id === moments[0]?.id || m.status !== 'ok');
    if (open.length) expanded.value = new Set([...expanded.value, ...open.map((m) => m.id)]);
}
absorb(props.moments);
watch(() => props.moments, absorb);

function toggle(id) {
    const s = new Set(expanded.value);
    s.has(id) ? s.delete(id) : s.add(id);
    expanded.value = s;
}

// Retry a stuck worklog post (#89). The re-dispatch is queued, so the reload
// that follows shows nothing new — the fresh run arrives on the next poll, once
// `queue:work` has picked the job up.
const retrying = ref(null);
function retry(run) {
    retrying.value = run.id;
    router.post(
        `/activity/runs/${run.id}/retry`,
        {},
        { preserveScroll: true, onFinish: () => (retrying.value = null) },
    );
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
            <div class="flex flex-none items-center gap-3">
                <!-- No socket, no updates: say so rather than showing a frozen list. -->
                <div
                    v-if="!online"
                    class="flex items-center gap-2 rounded-full bg-faint-2/10 px-3 py-1.5"
                    title="Not connected — this list won't update until the connection is back"
                >
                    <span class="h-2 w-2 rounded-full bg-faint-2"></span>
                    <span class="font-mono text-[11px] font-semibold text-faint-2">offline</span>
                </div>
                <div
                    v-if="failureCount"
                    class="flex items-center gap-2 rounded-full bg-behind/10 px-3 py-1.5"
                >
                    <span class="h-2 w-2 rounded-full bg-behind"></span>
                    <span class="font-mono text-[11px] font-semibold text-behind">{{ failureCount }} failed</span>
                </div>
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
                        <button
                            v-if="r.canRetry"
                            type="button"
                            class="flex-none rounded-full border border-divider-soft px-2.5 py-1 font-mono text-[10.5px] font-semibold text-muted transition-colors hover:text-fg disabled:opacity-50"
                            :disabled="retrying === r.id"
                            @click="retry(r)"
                        >
                            {{ retrying === r.id ? 'retrying…' : 'Retry' }}
                        </button>
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

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import Card from './Card.vue';
import EmptyState from './EmptyState.vue';

const props = defineProps({
    // { issue_id, key, title, accumulated_seconds, running, started_at, comment } | null
    active: { type: Object, default: null },
    // [{ issue_id, key, title, accumulated_seconds }]
    paused: { type: Array, default: () => [] },
});
const emit = defineEmits(['pause', 'resume', 'stop', 'comment']);

// One clock for the whole stack; drives live ticking of the running segment.
const now = ref(Date.now());
const tick = setInterval(() => (now.value = Date.now()), 1000);
onBeforeUnmount(() => clearInterval(tick));

// Local draft so the 1s tick re-render can't clobber in-progress typing.
// Reseeds when the active timer changes or its saved comment updates server-side.
const draft = ref(props.active?.comment ?? '');
watch(
    () => [props.active?.issue_id, props.active?.comment],
    () => (draft.value = props.active?.comment ?? ''),
);

function hms(seconds) {
    const s = Math.max(0, Math.floor(seconds));
    const p = (n) => String(n).padStart(2, '0');
    return `${p(Math.floor(s / 3600))}:${p(Math.floor((s / 60) % 60))}:${p(s % 60)}`;
}

function segSeconds(s) {
    const start = Date.parse(s.started_at);
    const end = s.ended_at ? Date.parse(s.ended_at) : now.value;
    return (end - start) / 1000;
}

const activeTime = computed(() => {
    if (!props.active) return '00:00:00';
    let s = props.active.accumulated_seconds;
    if (props.active.running && props.active.started_at) {
        s += (now.value - Date.parse(props.active.started_at)) / 1000;
    }
    return hms(s);
});
</script>

<template>
    <Card radius="24px" pad="26px 26px 30px">
        <div class="mb-1.5 flex items-center justify-between">
            <div class="font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">Timer stack</div>
            <div v-if="active" class="text-[12px] text-faint-2">{{ paused.length + 1 }} running</div>
        </div>

        <EmptyState v-if="!active" title="No timer running" text="Start a timer from an issue below." />

        <template v-else>
            <p class="mb-5 text-[12.5px] leading-[1.5] text-faint-2">
                Start a timer while one runs and it pushes on top. Stop it and the one beneath resumes.
            </p>

            <!-- active (top of stack) -->
            <div class="relative z-30 rounded-[18px] border-[1.5px] border-[#d9d3f4] bg-accent-wash px-5 py-[18px]">
                <div class="mb-3 flex items-center justify-between">
                    <div class="inline-flex items-center gap-2">
                        <span
                            class="h-2 w-2 rounded-full"
                            :class="active.running ? 'bg-accent' : 'bg-faint-2'"
                            :style="active.running ? 'animation: fyl-pulse 2s ease-in-out infinite' : ''"
                        ></span>
                        <span class="font-mono text-[11px] font-semibold uppercase tracking-[0.1em] text-accent-deep">{{
                            active.running ? 'Active' : 'Paused'
                        }}</span>
                    </div>
                    <span class="rounded-[7px] bg-accent-chip px-[9px] py-1 font-mono text-[12px] font-semibold text-accent">{{
                        active.key
                    }}</span>
                </div>
                <div class="mb-3.5 text-[15px] font-semibold tracking-[-0.01em]">{{ active.title }}</div>

                <input
                    v-model="draft"
                    placeholder="What are you working on?"
                    class="mb-3.5 w-full rounded-[11px] border border-border-soft bg-white px-3.5 py-2.5 text-[13px] outline-none focus:border-accent"
                    @change="$emit('comment', draft)"
                />

                <div class="flex items-end justify-between">
                    <span class="font-mono text-[32px] font-semibold tabular-nums tracking-[-0.02em] text-accent">{{
                        activeTime
                    }}</span>
                    <div class="flex gap-2">
                        <button
                            v-if="active.running"
                            class="flex h-10 w-10 cursor-pointer items-center justify-center rounded-[13px] border border-border-soft bg-white"
                            title="Pause"
                            @click="$emit('pause')"
                        >
                            <span class="block h-[11px] w-[3px] rounded-sm bg-accent"></span>
                            <span class="ml-[3px] block h-[11px] w-[3px] rounded-sm bg-accent"></span>
                        </button>
                        <button
                            v-else
                            class="flex h-10 w-10 cursor-pointer items-center justify-center rounded-[13px] border border-border-soft bg-white"
                            title="Resume"
                            @click="$emit('resume')"
                        >
                            <span class="ml-[2px] block h-0 w-0 border-y-[6px] border-l-[10px] border-y-transparent border-l-accent"></span>
                        </button>
                        <button
                            class="flex h-10 w-10 cursor-pointer items-center justify-center rounded-[13px] border-0 bg-accent shadow-btn"
                            title="Stop"
                            @click="$emit('stop')"
                        >
                            <span class="block h-[11px] w-[11px] rounded-sm bg-white"></span>
                        </button>
                    </div>
                </div>

                <!-- segments breakdown -->
                <div v-if="active.segments?.length" class="mt-4 border-t border-[#e3ddf5] pt-3">
                    <div class="mb-2 font-mono text-[10px] font-semibold uppercase tracking-[0.1em] text-faint-3">
                        {{ active.segments.length }} {{ active.segments.length === 1 ? 'segment' : 'segments' }}
                    </div>
                    <div
                        v-for="(seg, i) in active.segments"
                        :key="i"
                        class="flex items-baseline justify-between gap-3 py-1"
                    >
                        <div class="flex min-w-0 items-baseline gap-2">
                            <span class="font-mono text-[11px] font-medium text-faint-3">#{{ i + 1 }}</span>
                            <span
                                v-if="!seg.ended_at"
                                class="h-1.5 w-1.5 flex-none translate-y-[-1px] rounded-full bg-accent"
                                style="animation: fyl-pulse 2s ease-in-out infinite"
                            ></span>
                            <span v-if="seg.comment" class="truncate text-[12px] text-muted">{{ seg.comment }}</span>
                            <span v-else class="text-[12px] italic text-faint-3">no comment</span>
                        </div>
                        <span class="flex-none font-mono text-[12px] font-medium tabular-nums text-[#8a8578]">{{
                            hms(segSeconds(seg))
                        }}</span>
                    </div>
                </div>
            </div>

            <!-- paused, nested beneath (display-only, Q8) -->
            <div
                v-for="(row, i) in paused"
                :key="row.issue_id"
                class="relative rounded-b-2xl border border-t-0 border-border-soft px-4 pb-3.5 pt-[15px]"
                :class="i === 0 ? 'z-20 bg-surface-soft' : 'z-10 bg-[#f7f6f2] opacity-90'"
                :style="{ marginLeft: (i + 1) * 8 + 'px', marginRight: (i + 1) * 8 + 'px' }"
            >
                <div class="flex items-center justify-between">
                    <div class="flex min-w-0 items-center gap-2.5">
                        <span class="font-mono text-[11px] font-medium text-faint-3">paused</span>
                        <span class="font-mono text-[11px] font-semibold text-[#8a8578]">{{ row.key }}</span>
                        <span class="truncate text-[13px] font-medium text-muted">{{ row.title }}</span>
                    </div>
                    <span class="flex-none font-mono text-[14px] font-medium tabular-nums text-[#8a8578]">{{
                        hms(row.accumulated_seconds)
                    }}</span>
                </div>
            </div>
        </template>
    </Card>
</template>

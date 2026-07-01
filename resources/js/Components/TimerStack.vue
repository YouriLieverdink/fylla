<script setup>
import { computed, onBeforeUnmount, ref } from 'vue';
import Card from './Card.vue';
import EmptyState from './EmptyState.vue';

const props = defineProps({
    // { issue_id, key, title, accumulated_seconds, running, started_at, notes: [{at,text}] } | null
    active: { type: Object, default: null },
    // [{ issue_id, key, title, accumulated_seconds }]
    paused: { type: Array, default: () => [] },
});
const emit = defineEmits(['pause', 'resume', 'stop', 'note']);

// One clock for the whole stack; drives live ticking of the running segment.
const now = ref(Date.now());
const tick = setInterval(() => (now.value = Date.now()), 1000);
onBeforeUnmount(() => clearInterval(tick));

const noteDraft = ref('');
function addNote() {
    const text = noteDraft.value.trim();
    if (!text) return;
    emit('note', text);
    noteDraft.value = '';
}

function hms(seconds) {
    const s = Math.max(0, Math.floor(seconds));
    const p = (n) => String(n).padStart(2, '0');
    return `${p(Math.floor(s / 3600))}:${p(Math.floor((s / 60) % 60))}:${p(s % 60)}`;
}

const activeTime = computed(() => {
    if (!props.active) return '00:00:00';
    let s = props.active.accumulated_seconds;
    if (props.active.running && props.active.started_at) {
        s += (now.value - Date.parse(props.active.started_at)) / 1000;
    }
    return hms(s);
});

const notes = computed(() => props.active?.notes ?? []);
</script>

<template>
    <Card radius="24px" pad="26px 30px 28px">
        <div class="mb-4 flex items-center justify-between">
            <div class="font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">Timer stack</div>
            <div v-if="active" class="text-[12px] text-faint-2">{{ paused.length + 1 }} running · 1 active</div>
        </div>

        <EmptyState v-if="!active" title="No timer running" text="Start a timer from an issue below." />

        <template v-else>
            <!-- active card + paused stack (left) · notes (right) -->
            <div class="grid items-stretch gap-4 lg:grid-cols-[1fr_400px]">
                <!-- left column: active timer on top, paused nested beneath -->
                <div class="flex flex-col">
                <!-- active (top of stack) -->
                <div class="relative z-30 flex flex-col rounded-[18px] border-[1.5px] border-[#d9d3f4] bg-accent-wash px-[22px] py-5">
                    <div class="mb-3.5 flex items-center justify-between">
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
                    <div class="mb-auto text-[16px] font-semibold tracking-[-0.01em]">{{ active.title }}</div>

                    <div class="mt-[18px] flex items-end justify-between">
                        <span class="font-mono text-[36px] font-semibold tabular-nums tracking-[-0.02em] text-accent">{{
                            activeTime
                        }}</span>
                        <div class="flex gap-2">
                            <button
                                v-if="active.running"
                                class="flex h-[42px] w-[42px] cursor-pointer items-center justify-center rounded-[13px] border border-border-soft bg-white"
                                title="Pause"
                                @click="$emit('pause')"
                            >
                                <span class="block h-[11px] w-[3px] rounded-sm bg-accent"></span>
                                <span class="ml-[3px] block h-[11px] w-[3px] rounded-sm bg-accent"></span>
                            </button>
                            <button
                                v-else
                                class="flex h-[42px] w-[42px] cursor-pointer items-center justify-center rounded-[13px] border border-border-soft bg-white"
                                title="Resume"
                                @click="$emit('resume')"
                            >
                                <span class="ml-[2px] block h-0 w-0 border-y-[6px] border-l-[10px] border-y-transparent border-l-accent"></span>
                            </button>
                            <button
                                class="flex h-[42px] w-[42px] cursor-pointer items-center justify-center rounded-[13px] border-0 bg-accent shadow-btn"
                                title="Stop"
                                @click="$emit('stop')"
                            >
                                <span class="block h-3 w-3 rounded-sm bg-white"></span>
                            </button>
                        </div>
                    </div>
                </div>

                <!-- paused, nested beneath the active card (display-only, Q8) -->
                <div
                    v-for="(row, i) in paused"
                    :key="row.issue_id"
                    class="relative rounded-b-2xl border border-t-0 border-[#ebe7de] px-4 pb-3.5 pt-[15px]"
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

                <div v-if="paused.length" class="mt-3.5 text-center font-mono text-[11px] font-medium text-faint-3">
                    stop active → {{ paused[0].key }} resumes
                </div>
                </div>
                <!-- /left column -->

                <!-- notes on the running segment -->
                <div class="flex flex-col rounded-[18px] border border-[#ebe7de] bg-surface-soft px-[18px] py-4">
                    <div class="mb-3 flex items-center justify-between">
                        <span class="font-mono text-[10.5px] font-semibold uppercase tracking-[0.12em] text-faint">Notes · {{ active.key }}</span>
                        <span class="font-mono text-[11px] font-medium text-faint-3"
                            >{{ notes.length }} {{ notes.length === 1 ? 'note' : 'notes' }}</span
                        >
                    </div>

                    <div class="mb-3 flex max-h-[150px] min-h-[78px] flex-1 flex-col gap-2.5 overflow-auto">
                        <div v-for="(n, i) in notes" :key="i" class="flex items-start gap-2.5">
                            <span class="mt-px flex-none rounded-[6px] bg-accent-tint px-[7px] py-1 font-mono text-[11px] font-medium text-accent-soft">{{
                                n.at
                            }}</span>
                            <span class="min-w-0 break-words [overflow-wrap:anywhere] text-[12.5px] leading-[1.45] text-ink-soft">{{ n.text }}</span>
                        </div>
                        <div v-if="!notes.length" class="my-auto text-[12.5px] leading-[1.45] text-faint-3">
                            <template v-if="active.running">No notes yet — add one while the timer runs and it stamps the wall-clock time.</template>
                            <template v-else>Resume the timer to add notes.</template>
                        </div>
                    </div>

                    <div class="flex items-center gap-2">
                        <input
                            v-model="noteDraft"
                            type="text"
                            :disabled="!active.running"
                            :placeholder="active.running ? 'Add a note…' : 'Paused — resume to add notes'"
                            class="min-w-0 flex-1 rounded-[11px] border border-[#e0dbd0] bg-white px-3 py-2.5 text-[13px] outline-none focus:border-accent-tint-2 disabled:cursor-not-allowed disabled:bg-transparent disabled:text-faint-3"
                            @keydown.enter.prevent="addNote"
                        />
                        <button
                            :disabled="!active.running"
                            class="flex-none rounded-[11px] bg-accent px-[15px] py-2.5 font-sans text-[13px] font-semibold text-white shadow-btn disabled:cursor-not-allowed disabled:opacity-40"
                            @click="addNote"
                        >
                            Add
                        </button>
                    </div>
                </div>
            </div>
        </template>
    </Card>
</template>

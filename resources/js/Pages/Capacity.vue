<script setup>
import { computed, ref } from 'vue';
import { router } from '@inertiajs/vue3';
import Card from '../Components/Card.vue';
import AppHeader from '../Components/AppHeader.vue';

const props = defineProps({
    adjustments: { type: Array, default: () => [] },
    baseCapacity: { type: Number, default: 32 },
});

const opts = { preserveScroll: true };

// Form state. hours is a magnitude (1–24); the sign comes from `type`.
const type = ref('off'); // 'off' | 'extra'
const start = ref('');
const end = ref('');
const hours = ref(8);
const reason = ref('');
const editingId = ref(null);

const isOff = computed(() => type.value === 'off');

function pickOff() {
    type.value = 'off';
}
function pickExtra() {
    type.value = 'extra';
    end.value = '';
}
function inc() {
    hours.value = Math.min(24, hours.value + 1);
}
function dec() {
    hours.value = Math.max(1, hours.value - 1);
}

function reset() {
    editingId.value = null;
    start.value = '';
    end.value = '';
    reason.value = '';
    hours.value = 8;
}

function submit() {
    if (!start.value) return;
    const payload = { type: type.value, hours: hours.value, reason: reason.value };

    if (editingId.value) {
        router.patch('/capacity/' + editingId.value, payload, { ...opts, onSuccess: reset });
        return;
    }
    payload.start = start.value;
    if (isOff.value && end.value) payload.end = end.value;
    router.post('/capacity', payload, { ...opts, onSuccess: reset });
}

function editRow(row) {
    editingId.value = row.id;
    type.value = row.hours < 0 ? 'off' : 'extra';
    start.value = iso(row.date);
    end.value = iso(row.date);
    hours.value = Math.abs(row.hours);
    reason.value = row.reason ?? '';
}

function deleteRow(row) {
    router.delete('/capacity/' + row.id, {
        ...opts,
        onSuccess: () => {
            if (editingId.value === row.id) reset();
        },
    });
}

// dates arrive as ISO datetime strings from the model cast; keep the day only.
function iso(d) {
    return String(d).slice(0, 10);
}
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const DOW = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
function parse(d) {
    const [y, m, day] = iso(d).split('-').map(Number);
    return new Date(y, m - 1, day);
}
function dateMain(d) {
    const dt = parse(d);
    return DOW[dt.getDay()] + ' ' + dt.getDate() + ' ' + MONTHS[dt.getMonth()];
}
// compact span for a collapsed run; a/b are the oldest/newest ISO dates.
function rangeLabel(a, b) {
    const da = parse(a);
    const db = parse(b);
    const sameMonth = da.getMonth() === db.getMonth() && da.getFullYear() === db.getFullYear();
    return sameMonth
        ? da.getDate() + ' – ' + db.getDate() + ' ' + MONTHS[db.getMonth()]
        : da.getDate() + ' ' + MONTHS[da.getMonth()] + ' – ' + db.getDate() + ' ' + MONTHS[db.getMonth()];
}
// the weekday before `d` (skips Sat/Sun), so a run bridges the weekend gap.
function prevWeekday(d) {
    const dt = parse(d);
    do {
        dt.setDate(dt.getDate() - 1);
    } while (dt.getDay() === 0 || dt.getDay() === 6);
    return iso(dt.getFullYear() + '-' + String(dt.getMonth() + 1).padStart(2, '0') + '-' + String(dt.getDate()).padStart(2, '0'));
}

// Fold the (date-desc) rows into runs: same signed hours + reason, and each row
// the next weekday of the previous. Display-only — rows stay one per date.
const groups = computed(() => {
    const out = [];
    for (const r of props.adjustments) {
        const g = out[out.length - 1];
        const head = g && g.rows[0];
        const tail = g && g.rows[g.rows.length - 1];
        if (g && r.hours === head.hours && (r.reason || '') === (head.reason || '') && iso(r.date) === prevWeekday(tail.date)) {
            g.rows.push(r);
        } else {
            out.push({ rows: [r] });
        }
    }
    return out;
});

const expanded = ref({});
function toggle(id) {
    expanded.value[id] = !expanded.value[id];
}

const offCount = computed(() => props.adjustments.filter((r) => r.hours < 0).length);
const extraCount = computed(() => props.adjustments.filter((r) => r.hours > 0).length);
const pill = (n, one, many) => n + (n === 1 ? one : many);

const submitLabel = computed(() =>
    editingId.value ? 'Save changes' : isOff.value ? 'Add time off' : 'Add extra day',
);
const helperText = computed(() =>
    isOff.value
        ? 'A date range, expanded to weekdays only — weekends are skipped. Each day subtracts from that week’s capacity.'
        : 'A single date, any day of the week. Adds to that week’s capacity, banked toward vacation.',
);
const signPreview = computed(() => (isOff.value ? '−' : '+') + (Math.abs(hours.value) || 8) + 'h');
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-11">
        <!-- header -->
        <AppHeader />

        <!-- title + formula chip -->
        <div class="mb-8 flex items-end justify-between gap-8">
            <div>
                <h1 class="mb-3 text-[34px] font-bold leading-[1.05] tracking-[-0.03em]">Time off &amp; extra days</h1>
                <p class="max-w-[56ch] text-[15px] leading-[1.55] text-muted">
                    One signed adjustment per date against your contracted week.
                    <strong class="font-semibold text-ink-soft">Time off</strong> subtracts hours; an agreed
                    <strong class="font-semibold text-ink-soft">extra day</strong> adds them, banked toward vacation.
                </p>
            </div>
            <div class="flex-none rounded-[18px] border border-card-border bg-surface px-[22px] py-[18px] text-right shadow-card">
                <div class="mb-2.5 font-mono text-[10px] font-semibold uppercase tracking-[0.13em] text-faint">Weekly capacity</div>
                <div class="flex items-baseline justify-end gap-2 font-mono text-[15px] font-medium tabular-nums text-muted">
                    <span class="font-semibold text-ink">{{ baseCapacity }}h</span>
                    <span class="text-faint-4">base</span>
                    <span class="text-faint-4">±</span>
                    <span class="text-faint-4">Σ adjustments</span>
                </div>
            </div>
        </div>

        <!-- two-column layout -->
        <div class="grid items-start gap-[22px] lg:grid-cols-[400px_1fr]">
            <!-- form -->
            <Card radius="24px" pad="28px 30px" class="sticky top-6">
                <div class="mb-5 flex items-center justify-between">
                    <div class="font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">
                        {{ editingId ? 'Edit adjustment' : 'Add adjustment' }}
                    </div>
                    <button
                        v-if="editingId"
                        class="cursor-pointer px-1.5 py-1 text-[12px] font-semibold text-faint-2 hover:text-muted"
                        @click="reset"
                    >
                        Cancel
                    </button>
                </div>

                <!-- type toggle -->
                <div class="mb-5 flex gap-0.5 rounded-[14px] bg-sunken p-1">
                    <button
                        class="flex flex-1 cursor-pointer items-center justify-center gap-[7px] rounded-[11px] py-[11px] text-[13.5px] font-semibold transition"
                        :class="isOff ? 'bg-white text-ink shadow-[0_2px_6px_-2px_rgba(42,41,38,0.16)]' : 'text-[#8a8578]'"
                        @click="pickOff"
                    >
                        <svg width="13" height="13" viewBox="0 0 14 14" fill="none"><path d="M3 7h8" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" /></svg>
                        Time off
                    </button>
                    <button
                        class="flex flex-1 cursor-pointer items-center justify-center gap-[7px] rounded-[11px] py-[11px] text-[13.5px] font-semibold transition"
                        :class="!isOff ? 'bg-white text-ink shadow-[0_2px_6px_-2px_rgba(42,41,38,0.16)]' : 'text-[#8a8578]'"
                        @click="pickExtra"
                    >
                        <svg width="13" height="13" viewBox="0 0 14 14" fill="none"><path d="M7 3v8M3 7h8" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" /></svg>
                        Extra day
                    </button>
                </div>

                <!-- helper -->
                <div class="mb-5 flex items-start gap-[9px] rounded-[12px] border border-border-soft bg-surface-soft px-[13px] py-[11px]">
                    <span class="mt-[5px] h-1.5 w-1.5 flex-none rounded-full" :class="isOff ? 'bg-[#8a8578]' : 'bg-track'"></span>
                    <span class="text-[12.5px] leading-[1.45] text-muted">{{ helperText }}</span>
                </div>

                <!-- date fields -->
                <div v-if="isOff" class="mb-4 grid grid-cols-2 gap-3">
                    <label class="block">
                        <span class="mb-2 block font-mono text-[10.5px] font-semibold uppercase tracking-[0.1em] text-faint">From</span>
                        <input v-model="start" type="date" :disabled="!!editingId" class="fyl-date w-full rounded-[11px] border border-[#e0dbd0] bg-white px-3 py-[11px] font-mono text-[13px] font-medium text-ink outline-none focus:border-accent-tint-2 disabled:opacity-60" />
                    </label>
                    <label class="block">
                        <span class="mb-2 block font-mono text-[10.5px] font-semibold uppercase tracking-[0.1em] text-faint">To</span>
                        <input v-model="end" type="date" :disabled="!!editingId" class="fyl-date w-full rounded-[11px] border border-[#e0dbd0] bg-white px-3 py-[11px] font-mono text-[13px] font-medium text-ink outline-none focus:border-accent-tint-2 disabled:opacity-60" />
                    </label>
                </div>
                <div v-else class="mb-4">
                    <label class="block">
                        <span class="mb-2 block font-mono text-[10.5px] font-semibold uppercase tracking-[0.1em] text-faint">Date</span>
                        <input v-model="start" type="date" :disabled="!!editingId" class="fyl-date w-full rounded-[11px] border border-[#e0dbd0] bg-white px-3 py-[11px] font-mono text-[13px] font-medium text-ink outline-none focus:border-accent-tint-2 disabled:opacity-60" />
                    </label>
                </div>

                <!-- hours stepper -->
                <div class="mb-4">
                    <span class="mb-2 block font-mono text-[10.5px] font-semibold uppercase tracking-[0.1em] text-faint">Hours per day</span>
                    <div class="flex items-center gap-2.5">
                        <button class="flex h-[42px] w-[42px] flex-none cursor-pointer items-center justify-center rounded-[11px] border border-[#e0dbd0] bg-white text-[20px] text-muted transition hover:border-accent-tint-2 hover:bg-[#faf9fd]" @click="dec">−</button>
                        <div class="flex flex-1 items-baseline justify-center gap-1 rounded-[11px] border border-[#e0dbd0] bg-surface-soft py-[9px]">
                            <span class="font-mono text-[22px] font-semibold tabular-nums text-ink">{{ hours }}</span>
                            <span class="font-mono text-[13px] font-medium text-faint-2">h</span>
                        </div>
                        <button class="flex h-[42px] w-[42px] flex-none cursor-pointer items-center justify-center rounded-[11px] border border-[#e0dbd0] bg-white text-[20px] text-muted transition hover:border-accent-tint-2 hover:bg-[#faf9fd]" @click="inc">+</button>
                    </div>
                </div>

                <!-- reason -->
                <div class="mb-[22px]">
                    <span class="mb-2 block font-mono text-[10.5px] font-semibold uppercase tracking-[0.1em] text-faint">
                        Reason <span class="normal-case tracking-normal text-faint-4">· optional</span>
                    </span>
                    <input
                        v-model="reason"
                        type="text"
                        :placeholder="isOff ? 'Holiday, sick, PTO…' : 'Agreed extra day…'"
                        class="w-full rounded-[11px] border border-[#e0dbd0] bg-white px-3 py-[11px] text-[13.5px] text-ink outline-none placeholder:text-faint-3 focus:border-accent-tint-2"
                    />
                </div>

                <!-- preview + submit -->
                <div class="flex items-center gap-3">
                    <div class="flex-1 text-[12px] leading-[1.4] text-faint-2">
                        Stored as
                        <span class="font-mono text-[12px] font-semibold tabular-nums" :class="isOff ? 'text-behind' : 'text-track'">{{ signPreview }}</span>
                        {{ isOff ? ' per weekday' : ' on this date' }}
                    </div>
                    <button
                        class="flex-none cursor-pointer rounded-[13px] bg-accent px-[22px] py-[13px] text-[14px] font-semibold text-white shadow-btn"
                        @click="submit"
                    >
                        {{ submitLabel }}
                    </button>
                </div>
            </Card>

            <!-- list -->
            <Card radius="24px" pad="10px 10px 14px">
                <div class="flex items-center justify-between px-5 pb-3.5 pt-[18px]">
                    <div class="text-[16px] font-semibold tracking-[-0.01em]">All adjustments</div>
                    <div class="flex items-center gap-2">
                        <span class="rounded-full bg-divider px-[11px] py-1.5 font-mono text-[12px] font-medium text-[#8a8578]">{{ pill(offCount, ' day off', ' days off') }}</span>
                        <span class="rounded-full bg-track-tint px-[11px] py-1.5 font-mono text-[12px] font-medium text-track">{{ pill(extraCount, ' extra day', ' extra days') }}</span>
                    </div>
                </div>

                <div class="grid grid-cols-[150px_104px_1fr_76px] gap-3 px-5 py-2 font-mono text-[10px] font-semibold uppercase tracking-[0.1em] text-faint-3">
                    <span>Date</span><span>Adjustment</span><span>Reason</span><span></span>
                </div>

                <div v-if="adjustments.length" class="flex flex-col">
                    <template v-for="group in groups" :key="group.rows[0].id">
                        <!-- single date: a plain row -->
                        <div
                            v-if="group.rows.length === 1"
                            class="grid grid-cols-[150px_104px_1fr_76px] items-center gap-3 rounded-[14px] border-t border-divider-soft px-5 py-3.5"
                            :class="editingId === group.rows[0].id ? 'bg-accent-wash' : ''"
                        >
                            <div class="min-w-0">
                                <div class="whitespace-nowrap text-[14px] font-semibold">{{ dateMain(group.rows[0].date) }}</div>
                                <div class="mt-[3px] font-mono text-[11px] font-medium text-faint-3">{{ parse(group.rows[0].date).getFullYear() }}</div>
                            </div>
                            <div>
                                <span
                                    class="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 font-mono text-[12px] font-semibold tabular-nums"
                                    :class="group.rows[0].hours < 0 ? 'bg-divider text-muted' : 'bg-track-tint text-track'"
                                >
                                    {{ (group.rows[0].hours < 0 ? 'Off ' : 'Extra ') + (group.rows[0].hours > 0 ? '+' : '−') + Math.abs(group.rows[0].hours) }}
                                </span>
                            </div>
                            <div class="min-w-0 truncate text-[13.5px]" :class="group.rows[0].reason ? 'text-ink-soft' : 'text-faint-4'">
                                {{ group.rows[0].reason || '—' }}
                            </div>
                            <div class="flex justify-end gap-1">
                                <button title="Edit" class="flex h-8 w-8 cursor-pointer items-center justify-center rounded-[9px] border border-transparent transition hover:border-[#e0dbd0] hover:bg-[#faf9fd]" @click="editRow(group.rows[0])">
                                    <svg width="15" height="15" viewBox="0 0 16 16" fill="none"><path d="M11 2.5l2.5 2.5M3 13l7.5-7.5 2.5 2.5L5.5 15.5 2.5 16l.5-3z" stroke="#8a8578" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" /></svg>
                                </button>
                                <button title="Delete" class="flex h-8 w-8 cursor-pointer items-center justify-center rounded-[9px] border border-transparent transition hover:border-[#eccaca] hover:bg-[#fbf2f1]" @click="deleteRow(group.rows[0])">
                                    <svg width="15" height="15" viewBox="0 0 16 16" fill="none"><path d="M3 4.5h10M6.5 4V2.8h3V4M4.2 4.5l.6 9h6.4l.6-9" stroke="#b5877a" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" /></svg>
                                </button>
                            </div>
                        </div>

                        <!-- run: one collapsible summary row -->
                        <template v-else>
                            <div
                                class="grid cursor-pointer grid-cols-[150px_104px_1fr_76px] items-center gap-3 rounded-[14px] border-t border-divider-soft px-5 py-3.5 hover:bg-surface-soft"
                                @click="toggle(group.rows[0].id)"
                            >
                                <div class="min-w-0">
                                    <div class="whitespace-nowrap text-[14px] font-semibold">{{ rangeLabel(group.rows[group.rows.length - 1].date, group.rows[0].date) }}</div>
                                    <div class="mt-[3px] font-mono text-[11px] font-medium text-faint-3">{{ parse(group.rows[0].date).getFullYear() }} · {{ group.rows.length }} days</div>
                                </div>
                                <div>
                                    <span
                                        class="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 font-mono text-[12px] font-semibold tabular-nums"
                                        :class="group.rows[0].hours < 0 ? 'bg-divider text-muted' : 'bg-track-tint text-track'"
                                    >
                                        {{ (group.rows[0].hours < 0 ? 'Off ' : 'Extra ') + (group.rows[0].hours > 0 ? '+' : '−') + Math.abs(group.rows[0].hours) }}
                                    </span>
                                </div>
                                <div class="min-w-0 truncate text-[13.5px]" :class="group.rows[0].reason ? 'text-ink-soft' : 'text-faint-4'">
                                    {{ group.rows[0].reason || '—' }}
                                </div>
                                <div class="flex justify-end pr-1.5 text-faint-2">
                                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" class="transition-transform" :class="expanded[group.rows[0].id] ? 'rotate-90' : ''">
                                        <path d="M5 3l3.5 3.5L5 10" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
                                    </svg>
                                </div>
                            </div>

                            <!-- expanded members: edit/delete per date -->
                            <template v-if="expanded[group.rows[0].id]">
                                <div
                                    v-for="row in group.rows"
                                    :key="row.id"
                                    class="grid grid-cols-[150px_104px_1fr_76px] items-center gap-3 rounded-[14px] bg-surface-soft py-2.5 pl-9 pr-5"
                                    :class="editingId === row.id ? '!bg-accent-wash' : ''"
                                >
                                    <div class="min-w-0 whitespace-nowrap text-[13px] font-medium text-muted">{{ dateMain(row.date) }}</div>
                                    <div></div>
                                    <div class="min-w-0 truncate text-[13px]" :class="row.reason ? 'text-muted' : 'text-faint-4'">{{ row.reason || '—' }}</div>
                                    <div class="flex justify-end gap-1">
                                        <button title="Edit" class="flex h-8 w-8 cursor-pointer items-center justify-center rounded-[9px] border border-transparent transition hover:border-[#e0dbd0] hover:bg-white" @click="editRow(row)">
                                            <svg width="14" height="14" viewBox="0 0 16 16" fill="none"><path d="M11 2.5l2.5 2.5M3 13l7.5-7.5 2.5 2.5L5.5 15.5 2.5 16l.5-3z" stroke="#8a8578" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" /></svg>
                                        </button>
                                        <button title="Delete" class="flex h-8 w-8 cursor-pointer items-center justify-center rounded-[9px] border border-transparent transition hover:border-[#eccaca] hover:bg-white" @click="deleteRow(row)">
                                            <svg width="14" height="14" viewBox="0 0 16 16" fill="none"><path d="M3 4.5h10M6.5 4V2.8h3V4M4.2 4.5l.6 9h6.4l.6-9" stroke="#b5877a" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" /></svg>
                                        </button>
                                    </div>
                                </div>
                            </template>
                        </template>
                    </template>
                </div>

                <div v-else class="px-7 py-[52px] text-center">
                    <div class="mx-auto mb-4 flex h-[52px] w-[52px] items-center justify-center rounded-[16px] border border-border-soft bg-canvas">
                        <svg width="22" height="22" viewBox="0 0 24 24" fill="none"><rect x="4" y="5" width="16" height="15" rx="2.5" stroke="#c2bdb1" stroke-width="1.5" /><path d="M4 9.5h16M8 3v4M16 3v4" stroke="#c2bdb1" stroke-width="1.5" stroke-linecap="round" /></svg>
                    </div>
                    <div class="mb-1.5 text-[15px] font-semibold">No adjustments yet</div>
                    <div class="mx-auto max-w-[38ch] text-[13px] leading-[1.55] text-faint-2">
                        Add time off or an extra day on the left. Every date sits at your contracted {{ baseCapacity }}h until then.
                    </div>
                </div>
            </Card>
        </div>
    </div>
</template>

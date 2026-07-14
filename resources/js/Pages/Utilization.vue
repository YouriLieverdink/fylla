<script setup>
import { computed, ref } from 'vue';
import Card from '../Components/Card.vue';
import AppHeader from '../Components/AppHeader.vue';
import SegmentedControl from '../Components/SegmentedControl.vue';

const props = defineProps({
    report: { type: Object, required: true }, // { weeks, totals, target, softFloor }
    windowWeeks: { type: Number, default: 13 },
    entries: { type: Array, default: () => [] },
});

const totals = computed(() => props.report.totals);

const view = ref('Weekly breakdown');

// Same target band as the dashboard: at/above target reads billable (green),
// within the soft band neutral, below the floor behind (red). null = no data.
function utilClass(v) {
    if (v == null) return 'text-faint-4';
    if (v >= props.report.target) return 'text-track';
    if (v >= props.report.softFloor) return 'text-ink';
    return 'text-behind';
}
const fmtPct = (v) => (v == null ? '—' : v + '%');

// Signed adjustment → chip label, e.g. -8 → "Off −8", 8 → "Extra +8".
const chipLabel = (h) => (h < 0 ? 'Off ' : 'Extra ') + (h > 0 ? '+' : '−') + Math.abs(h);

// minutes → "1h 30m" / "45m" / "2h".
function hm(min) {
    const h = Math.floor(min / 60);
    const m = min % 60;
    return (h ? h + 'h' : '') + (h && m ? ' ' : '') + (m || !h ? m + 'm' : '');
}

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const DOW = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
function parseDate(d) {
    const [y, m, day] = String(d).slice(0, 10).split('-').map(Number);
    return new Date(y, m - 1, day);
}
function entryDate(d) {
    const dt = parseDate(d);
    return DOW[dt.getDay()] + ' ' + dt.getDate() + ' ' + MONTHS[dt.getMonth()];
}

// Monday of the week containing dt, matching PHP startOfWeek(MONDAY).
function monday(dt) {
    const d = new Date(dt.getFullYear(), dt.getMonth(), dt.getDate());
    const isoDow = d.getDay() === 0 ? 7 : d.getDay();
    d.setDate(d.getDate() - (isoDow - 1));
    return d;
}
const curMonKey = monday(new Date()).toDateString();

// Entries arrive newest-first, so same-week rows are already adjacent: fold
// them into week sections with an hours subtotal. Labels match the breakdown
// table's "Mon j" (e.g. "Jul 13").
const weekGroups = computed(() => {
    const out = [];
    for (const e of props.entries) {
        const m = monday(parseDate(e.date));
        const key = m.toDateString();
        let g = out[out.length - 1];
        if (!g || g.key !== key) {
            g = { key, label: MONTHS[m.getMonth()] + ' ' + m.getDate(), minutes: 0, entries: [], isCurrent: key === curMonKey };
            out.push(g);
        }
        g.entries.push(e);
        g.minutes += e.minutes;
    }
    return out;
});

// Current week open by default; others collapsed. Toggle overrides.
const open = ref({});
const isOpen = (g) => open.value[g.key] ?? g.isCurrent;
const toggle = (g) => (open.value[g.key] = !isOpen(g));
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-11">
        <AppHeader />

        <div class="mb-8">
            <h1 class="mb-3 text-[34px] font-bold leading-[1.05] tracking-[-0.03em]">Utilization detail</h1>
            <p class="max-w-[62ch] text-[15px] leading-[1.55] text-muted">
                The data behind the headline. Utilization is
                <strong class="font-semibold text-ink-soft">billable ÷ capacity</strong> over the rolling
                {{ windowWeeks }}-week window, current week prorated — the same number as the
                <strong class="font-semibold text-ink-soft">Personal</strong> dashboard.
            </p>
        </div>

        <!-- window totals -->
        <Card radius="24px" pad="28px 30px" accent class="mb-[22px]">
            <div class="mb-6 font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">
                Window totals · {{ windowWeeks }} weeks
            </div>
            <div class="grid grid-cols-2 gap-y-6 sm:grid-cols-5">
                <div>
                    <div class="mb-1.5 font-mono text-[10.5px] font-semibold uppercase tracking-[0.1em] text-faint-3">Capacity</div>
                    <div class="font-mono text-[26px] font-semibold tabular-nums text-ink">{{ totals.capacity }}h</div>
                </div>
                <div>
                    <div class="mb-1.5 font-mono text-[10.5px] font-semibold uppercase tracking-[0.1em] text-faint-3">Worked</div>
                    <div class="font-mono text-[26px] font-semibold tabular-nums text-ink">{{ totals.worked }}h</div>
                </div>
                <div>
                    <div class="mb-1.5 font-mono text-[10.5px] font-semibold uppercase tracking-[0.1em] text-faint-3">Billable</div>
                    <div class="font-mono text-[26px] font-semibold tabular-nums text-ink">{{ totals.billable }}h</div>
                </div>
                <div>
                    <div class="mb-1.5 font-mono text-[10.5px] font-semibold uppercase tracking-[0.1em] text-faint-3">Billable share</div>
                    <div class="font-mono text-[26px] font-semibold tabular-nums text-ink">{{ fmtPct(totals.billableShare) }}</div>
                </div>
                <div>
                    <div class="mb-1.5 font-mono text-[10.5px] font-semibold uppercase tracking-[0.1em] text-faint-3">Utilization</div>
                    <div class="font-mono text-[26px] font-semibold tabular-nums text-accent">{{ fmtPct(totals.utilization) }}</div>
                </div>
            </div>
        </Card>

        <!-- view switcher: weekly breakdown ⇆ time entries -->
        <div class="mb-[22px]">
            <SegmentedControl v-model="view" :options="['Weekly breakdown', 'Time entries']" />
        </div>

        <!-- weekly breakdown -->
        <Card v-if="view === 'Weekly breakdown'" radius="24px" pad="10px 10px 14px" class="mb-[22px]">
            <div class="flex items-center justify-between px-5 pb-3.5 pt-[18px]">
                <div class="text-[16px] font-semibold tracking-[-0.01em]">Weekly breakdown</div>
                <span class="rounded-full bg-surface-soft px-[11px] py-1.5 font-mono text-[11px] font-medium text-faint-2">
                    billable ÷ capacity · current week prorated
                </span>
            </div>

            <div class="grid grid-cols-[repeat(6,1fr)_2fr] gap-3 px-5 py-2 font-mono text-[10px] font-semibold uppercase tracking-[0.1em] text-faint-3">
                <span>Week</span><span class="text-right">Capacity</span><span class="text-right">Worked</span><span class="text-right">Billable</span><span class="text-right">Billable share</span><span class="text-right">Utilization</span><span class="pl-5">Adjustments</span>
            </div>

            <div class="flex flex-col">
                <div
                    v-for="(w, i) in report.weeks"
                    :key="w.label"
                    class="grid grid-cols-[repeat(6,1fr)_2fr] items-center gap-3 rounded-[14px] border-t border-divider-soft px-5 py-3"
                    :class="i === 0 ? 'bg-accent-wash' : ''"
                >
                    <div class="text-[13.5px] font-semibold">
                        {{ w.label }}<span v-if="i === 0" class="mt-[3px] block font-mono text-[11px] font-medium text-faint-3">this week</span>
                    </div>
                    <div class="text-right font-mono text-[13.5px] tabular-nums text-muted">{{ w.capacity }}h</div>
                    <div class="text-right font-mono text-[13.5px] tabular-nums text-muted">{{ w.worked }}h</div>
                    <div class="text-right font-mono text-[13.5px] tabular-nums text-muted">{{ w.billable }}h</div>
                    <div class="text-right font-mono text-[13.5px] tabular-nums text-muted">{{ fmtPct(w.billableShare) }}</div>
                    <div class="text-right font-mono text-[14px] font-semibold tabular-nums" :class="utilClass(w.utilization)">
                        {{ fmtPct(w.utilization) }}
                    </div>
                    <div v-if="w.adjustments.length" class="flex flex-wrap gap-1.5 pl-5">
                        <span
                            v-for="c in w.adjustments"
                            :key="c.hours"
                            class="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1 font-mono text-[12px] font-semibold tabular-nums"
                            :class="c.hours < 0 ? 'bg-divider text-muted' : 'bg-track-tint text-track'"
                        >
                            {{ chipLabel(c.hours) }}<span v-if="c.count > 1" class="text-faint-3">×{{ c.count }}</span>
                        </span>
                    </div>
                    <div v-else class="pl-5 text-[12.5px] text-faint-4">—</div>
                </div>
            </div>
        </Card>

        <!-- time entries -->
        <Card v-if="view === 'Time entries'" radius="24px" pad="10px 10px 14px">
            <div class="flex items-center justify-between px-5 pb-3.5 pt-[18px]">
                <div class="text-[16px] font-semibold tracking-[-0.01em]">Time entries</div>
                <span class="rounded-full bg-divider px-[11px] py-1.5 font-mono text-[12px] font-medium text-[#8a8578]">
                    {{ entries.length }}{{ entries.length === 1 ? ' entry' : ' entries' }}
                </span>
            </div>

            <div class="grid grid-cols-[120px_1fr_170px_64px] gap-3 px-5 py-2 font-mono text-[10px] font-semibold uppercase tracking-[0.1em] text-faint-3">
                <span>Date</span><span>Issue</span><span>Project</span><span class="text-right">Time</span>
            </div>

            <div v-if="entries.length" class="flex flex-col">
                <template v-for="g in weekGroups" :key="g.key">
                    <!-- week header: click to expand -->
                    <button
                        class="grid cursor-pointer grid-cols-[1fr_auto] items-center gap-3 border-t border-divider-soft px-5 py-3 text-left hover:bg-surface-soft"
                        :class="g.isCurrent ? 'bg-accent-wash' : ''"
                        @click="toggle(g)"
                    >
                        <div class="flex items-center gap-2.5">
                            <svg width="12" height="12" viewBox="0 0 14 14" fill="none" class="text-faint-2 transition-transform" :class="isOpen(g) ? 'rotate-90' : ''">
                                <path d="M5 3l3.5 3.5L5 10" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
                            </svg>
                            <span class="text-[13.5px] font-semibold">{{ g.label }}</span>
                            <span v-if="g.isCurrent" class="font-mono text-[11px] font-medium text-faint-3">this week</span>
                        </div>
                        <div class="flex items-baseline gap-2.5 font-mono text-[12px] tabular-nums text-faint-2">
                            <span class="font-semibold text-muted">{{ hm(g.minutes) }}</span>
                            <span>{{ g.entries.length }}{{ g.entries.length === 1 ? ' entry' : ' entries' }}</span>
                        </div>
                    </button>

                    <!-- week's entries -->
                    <div
                        v-for="e in isOpen(g) ? g.entries : []"
                        :key="e.id"
                        class="grid grid-cols-[120px_1fr_170px_64px] items-start gap-3 bg-surface-soft px-5 py-3"
                    >
                        <div class="whitespace-nowrap text-[13px] font-medium text-muted">{{ entryDate(e.date) }}</div>
                        <div class="min-w-0">
                            <div class="truncate text-[13.5px] font-semibold text-ink">
                                <span v-if="e.issueKey" class="font-mono text-[12px] text-faint-2">{{ e.issueKey }}</span>
                                {{ e.issueTitle || '—' }}
                            </div>
                            <div v-if="e.note" class="mt-0.5 whitespace-pre-line break-words text-[12.5px] text-faint-2">{{ e.note }}</div>
                        </div>
                        <div class="flex min-w-0 flex-col gap-1.5">
                            <span class="truncate text-[12.5px] text-muted">{{ e.project || '—' }}</span>
                            <span
                                class="inline-flex w-fit items-center rounded-md px-2 py-0.5 font-mono text-[11px] font-semibold"
                                :class="e.billable ? 'bg-track-tint text-track' : 'bg-divider text-muted'"
                            >
                                {{ e.billable ? 'billable' : 'internal' }}
                            </span>
                        </div>
                        <div class="text-right font-mono text-[13.5px] tabular-nums text-ink">{{ hm(e.minutes) }}</div>
                    </div>
                </template>
            </div>

            <div v-else class="px-7 py-[52px] text-center">
                <div class="mb-1.5 text-[15px] font-semibold">No time entries in this window</div>
                <div class="mx-auto max-w-[38ch] text-[13px] leading-[1.55] text-faint-2">
                    Worklogs synced from Kendo over the last {{ windowWeeks }} weeks will appear here.
                </div>
            </div>
        </Card>
    </div>
</template>

<script setup>
import { computed, ref } from 'vue';
import { router } from '@inertiajs/vue3';
import Card from '../Components/Card.vue';
import AppHeader from '../Components/AppHeader.vue';
import CalendarGrid from '../Components/CalendarGrid.vue';
import CellEditor from '../Components/CellEditor.vue';
import { usePageCursor } from '../Composables/usePageCursor';

const props = defineProps({
    year: { type: Number, required: true },
    years: { type: Array, default: () => [] },
    adjustments: { type: Array, default: () => [] },
    accrual: { default: null },
    ledger: { type: Object, required: true },
    overview: { type: Array, default: () => [] },
    baseCapacity: { type: Number, default: 32 },
    offWeekday: { type: Number, default: 5 },
});

const opts = { preserveScroll: true };

// Dutch-style number: 2 dp, trailing zeros trimmed, dot → comma.
function nf(n) {
    return String(Math.round(Number(n) * 100) / 100).replace('.', ',');
}
function signed(n) {
    const v = Number(n);
    if (v === 0) return '0';
    return (v > 0 ? '+' : '−') + nf(Math.abs(v));
}

function pickYear(y) {
    router.get('/capacity', { year: y }, { preserveState: false, preserveScroll: true });
}

// ── Ledger accrual (inline edit) ──────────────────────────────────────────
const editingAccrual = ref(false);
const accrualInput = ref('');
function editAccrual() {
    accrualInput.value = props.accrual != null ? nf(props.accrual) : '';
    editingAccrual.value = true;
}
function saveAccrual() {
    const h = Number(String(accrualInput.value).replace(',', '.'));
    if (Number.isNaN(h)) {
        editingAccrual.value = false;
        return;
    }
    router.post('/capacity/accrual', { year: props.year, hours: h }, {
        ...opts,
        onSuccess: () => (editingAccrual.value = false),
    });
}

// The planned (still-to-confirm) portion already folded into the balance. The
// balance counts planned + confirmed alike, so this is what's penciled in but
// not yet entered into the official leave system. Holidays don't touch the
// ledger, so this is banked + taken planned sub-sums only.
const bankedPlanned = computed(() => Number(props.ledger.bankedPlanned));
const takenPlanned = computed(() => Number(props.ledger.takenPlanned));
const plannedInBalance = computed(() => bankedPlanned.value + takenPlanned.value);

// ── Editor popover ────────────────────────────────────────────────────────
const editor = ref({ open: false, x: 0, y: 0, start: '', end: '', existing: null });
function onSelect(sel) {
    editor.value = { open: true, ...sel };
}
function closeEditor() {
    editor.value.open = false;
}
function save(payload) {
    if (payload.id) {
        router.patch('/capacity/' + payload.id, payload, { ...opts, onSuccess: closeEditor });
    } else {
        router.post('/capacity', payload, { ...opts, onSuccess: closeEditor });
    }
}
function remove(id) {
    router.delete('/capacity/' + id, { ...opts, onSuccess: closeEditor });
}

// ── Trips list: fold consecutive same-type/reason weekday runs ────────────
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
function parse(d) {
    const [y, m, day] = String(d).slice(0, 10).split('-').map(Number);
    return new Date(y, m - 1, day);
}
function iso(d) {
    return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0');
}
// The next working day after d (skips weekends + the contracted off-day).
function nextWorkday(d) {
    const dt = parse(d);
    do {
        dt.setDate(dt.getDate() + 1);
    } while (dt.getDay() === 0 || dt.getDay() === 6 || (dt.getDay() === 0 ? 7 : dt.getDay()) === props.offWeekday);
    return iso(dt);
}
function dayLabel(d) {
    const dt = parse(d);
    return dt.getDate() + ' ' + MONTHS[dt.getMonth()];
}

const trips = computed(() => {
    const out = [];
    for (const a of props.adjustments) {
        const g = out[out.length - 1];
        const tail = g && g.rows[g.rows.length - 1];
        if (
            g &&
            g.type === a.type &&
            (g.reason || '') === (a.reason || '') &&
            g.status === a.status &&
            String(a.date).slice(0, 10) === nextWorkday(tail.date)
        ) {
            g.rows.push(a);
        } else {
            out.push({ type: a.type, reason: a.reason, status: a.status, rows: [a] });
        }
    }
    return out;
});

const typeMeta = {
    off: { label: 'Time off', chip: 'bg-off-tint text-off' },
    holiday: { label: 'Holiday', chip: 'bg-holiday-tint text-holiday' },
    sick: { label: 'Sick', chip: 'bg-sick-tint text-sick' },
    extra: { label: 'Extra day', chip: 'bg-track-tint text-track' },
};
function tripRange(t) {
    const a = t.rows[0].date;
    const b = t.rows[t.rows.length - 1].date;
    return t.rows.length === 1 ? dayLabel(a) : dayLabel(a) + ' – ' + dayLabel(b);
}
function tripHours(t) {
    return t.rows.reduce((s, r) => s + Math.abs(Number(r.hours)), 0);
}

// j/k cursor over the ledger + calendar cards, then trip rows, then overview
// rows (#43).
const focusTargets = computed(() => [
    'ledger',
    'calendar',
    ...trips.value.map((_, i) => 'trip-' + i),
    ...props.overview.map((r) => 'ovr-' + r.year),
]);
const cursor = usePageCursor(() => focusTargets.value);
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-11">
        <AppHeader />

        <!-- title -->
        <div class="mb-8">
            <h1 class="mb-3 text-[34px] font-bold leading-[1.05] tracking-[-0.03em]">Time off &amp; vacation</h1>
            <p class="max-w-[62ch] text-[15px] leading-[1.55] text-muted">
                A year at a glance. Click a day or drag a range to plan
                <strong class="font-semibold text-ink-soft">time off</strong>, a
                <strong class="font-semibold text-ink-soft">holiday</strong>, or an
                <strong class="font-semibold text-ink-soft">extra day</strong>. The ledger tracks your running vacation
                balance in hours.
            </p>
        </div>

        <!-- ledger panel -->
        <Card radius="24px" pad="26px 30px" data-row="ledger" class="mb-[22px] scroll-my-12" :class="cursor.isActive('ledger') && 'ring-2 ring-accent'">
            <div class="flex flex-wrap items-end justify-between gap-8">
                <!-- equation -->
                <div class="flex flex-wrap items-end gap-x-6 gap-y-4">
                    <div>
                        <div class="mb-1.5 font-mono text-[10px] font-semibold uppercase tracking-[0.11em] text-faint">Carryover</div>
                        <div class="font-mono text-[19px] font-semibold tabular-nums text-muted">{{ nf(ledger.carryover) }}<span class="text-[13px] text-faint-3">h</span></div>
                    </div>
                    <div class="pb-1 text-[16px] font-medium text-faint-4">+</div>
                    <div>
                        <div class="mb-1.5 flex items-center gap-1.5 font-mono text-[10px] font-semibold uppercase tracking-[0.11em] text-faint">
                            Accrual
                            <svg v-if="!editingAccrual" width="11" height="11" viewBox="0 0 16 16" fill="none" class="cursor-pointer" @click="editAccrual"><path d="M11 2.5l2.5 2.5M3 13l7.5-7.5 2.5 2.5L5.5 15.5 2.5 16l.5-3z" stroke="#b6b1a6" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" /></svg>
                        </div>
                        <input
                            v-if="editingAccrual"
                            v-model="accrualInput"
                            type="text"
                            inputmode="decimal"
                            autofocus
                            class="w-[86px] rounded-[9px] border border-accent-tint-2 bg-white px-2 py-1 font-mono text-[18px] font-semibold tabular-nums text-ink outline-none"
                            @keydown.enter="saveAccrual"
                            @blur="saveAccrual"
                        />
                        <button
                            v-else
                            class="cursor-pointer font-mono text-[19px] font-semibold tabular-nums text-muted hover:text-ink"
                            @click="editAccrual"
                        >
                            {{ accrual != null ? nf(accrual) : '—' }}<span class="text-[13px] text-faint-3">h</span>
                        </button>
                    </div>
                    <div class="pb-1 text-[16px] font-medium text-faint-4">+</div>
                    <div>
                        <div class="mb-1.5 font-mono text-[10px] font-semibold uppercase tracking-[0.11em] text-faint">Banked extra</div>
                        <div class="font-mono text-[19px] font-semibold tabular-nums text-track">{{ signed(ledger.banked) }}<span class="text-[13px] text-faint-3">h</span></div>
                        <div v-if="bankedPlanned !== 0" class="mt-1 font-mono text-[10px] text-behind">of which {{ signed(bankedPlanned) }} planned</div>
                    </div>
                    <div class="pb-1 text-[16px] font-medium text-faint-4">+</div>
                    <div>
                        <div class="mb-1.5 font-mono text-[10px] font-semibold uppercase tracking-[0.11em] text-faint">Taken</div>
                        <div class="font-mono text-[19px] font-semibold tabular-nums text-muted">{{ signed(ledger.taken) }}<span class="text-[13px] text-faint-3">h</span></div>
                        <div v-if="takenPlanned !== 0" class="mt-1 font-mono text-[10px] text-behind">of which {{ signed(takenPlanned) }} planned</div>
                    </div>
                    <div class="pb-1 text-[16px] font-medium text-faint-4">=</div>
                </div>

                <!-- balance hero -->
                <div class="text-right">
                    <div class="mb-1 font-mono text-[10px] font-semibold uppercase tracking-[0.13em] text-faint">Balance {{ year }}</div>
                    <div class="font-mono text-[38px] font-bold leading-none tabular-nums tracking-[-0.02em] text-ink">
                        {{ nf(ledger.balance) }}<span class="text-[22px] text-faint-3">h</span>
                    </div>
                    <div class="mt-2 font-mono text-[13px] font-medium tabular-nums text-muted">
                        {{ nf(ledger.days) }} days · {{ nf(ledger.weeks) }} weeks left
                    </div>
                    <div v-if="plannedInBalance !== 0" class="mt-1 font-mono text-[12px] font-medium tabular-nums text-behind">
                        incl. {{ signed(plannedInBalance) }}h still planned · {{ nf(Number(ledger.balance) - plannedInBalance) }}h confirmed
                    </div>
                </div>
            </div>
        </Card>

        <!-- year switcher -->
        <div class="mb-[22px] inline-flex gap-0.5 rounded-[14px] bg-sunken p-1">
            <button
                v-for="y in years"
                :key="y"
                class="cursor-pointer rounded-[11px] px-[18px] py-2 font-mono text-[13px] font-semibold tabular-nums transition"
                :class="y === year ? 'bg-surface text-ink shadow-[0_2px_6px_-2px_rgba(42,41,38,0.14)]' : 'text-[#8a8578]'"
                @click="pickYear(y)"
            >
                {{ y }}
            </button>
        </div>

        <!-- calendar grid -->
        <Card radius="24px" pad="22px 24px" data-row="calendar" class="mb-[22px] scroll-my-12" :class="cursor.isActive('calendar') && 'ring-2 ring-accent'">
            <CalendarGrid :year="year" :adjustments="adjustments" :off-weekday="offWeekday" @select="onSelect" />

            <!-- legend -->
            <div class="mt-5 flex flex-wrap items-center gap-x-5 gap-y-2 border-t border-divider pt-4 font-mono text-[11px] text-faint-2">
                <span class="flex items-center gap-1.5"><span class="h-3 w-3 rounded-[4px] bg-off"></span> Time off</span>
                <span class="flex items-center gap-1.5"><span class="h-3 w-3 rounded-[4px] bg-sick"></span> Sick</span>
                <span class="flex items-center gap-1.5"><span class="h-3 w-3 rounded-[4px] bg-holiday"></span> Holiday</span>
                <span class="flex items-center gap-1.5"><span class="h-3 w-3 rounded-[4px] bg-track"></span> Extra day</span>
                <span class="flex items-center gap-1.5"><span class="h-3 w-3 rounded-[4px] bg-surface ring-1 ring-inset ring-faint-3"></span> Planned (hollow)</span>
                <span class="flex items-center gap-1.5"><span class="h-3 w-3 rounded-[4px] bg-sunken"></span> Non-working</span>
            </div>
        </Card>

        <!-- trips + overview -->
        <div class="grid items-start gap-[22px] lg:grid-cols-[1fr_1fr]">
            <!-- trips this year -->
            <Card radius="24px" pad="10px 10px 14px">
                <div class="px-5 pb-3 pt-[18px] text-[16px] font-semibold tracking-[-0.01em]">Trips &amp; days · {{ year }}</div>
                <div v-if="trips.length" class="flex flex-col">
                    <div
                        v-for="(t, i) in trips"
                        :key="i"
                        :data-row="'trip-' + i"
                        class="flex scroll-my-12 items-center gap-3 rounded-[12px] border-t border-divider-soft px-5 py-3"
                        :class="cursor.isActive('trip-' + i) && 'ring-2 ring-inset ring-accent'"
                    >
                        <span class="w-[120px] flex-none whitespace-nowrap font-mono text-[13px] font-semibold tabular-nums text-ink-soft">{{ tripRange(t) }}</span>
                        <span class="flex-none rounded-md px-2 py-1 font-mono text-[10.5px] font-semibold" :class="typeMeta[t.type].chip">{{ typeMeta[t.type].label }}</span>
                        <span class="min-w-0 flex-1 truncate text-[13.5px]" :class="t.reason ? 'text-muted' : 'text-faint-4'">{{ t.reason || '—' }}</span>
                        <span class="flex-none font-mono text-[12px] tabular-nums text-faint-2">{{ nf(tripHours(t)) }}h</span>
                        <span v-if="t.status === 'planned'" class="flex-none font-mono text-[10px] font-semibold uppercase tracking-[0.08em] text-behind">planned</span>
                    </div>
                </div>
                <div v-else class="px-5 py-10 text-center text-[13px] text-faint-2">
                    Nothing booked for {{ year }} yet. Click a day in the grid to start.
                </div>
            </Card>

            <!-- multi-year overzicht -->
            <Card radius="24px" pad="10px 10px 14px">
                <div class="px-5 pb-3 pt-[18px] text-[16px] font-semibold tracking-[-0.01em]">Overzicht · balance by year</div>
                <div class="grid grid-cols-[1fr_repeat(4,minmax(0,1fr))] gap-2 px-5 py-2 font-mono text-[9.5px] font-semibold uppercase tracking-[0.08em] text-faint-3">
                    <span>Year</span><span class="text-right">Accrual</span><span class="text-right">Banked</span><span class="text-right">Taken</span><span class="text-right">Balance</span>
                </div>
                <button
                    v-for="row in overview"
                    :key="row.year"
                    :data-row="'ovr-' + row.year"
                    class="grid w-full cursor-pointer scroll-my-12 grid-cols-[1fr_repeat(4,minmax(0,1fr))] gap-2 rounded-[10px] border-t border-divider-soft px-5 py-2.5 text-right font-mono text-[13px] tabular-nums transition hover:bg-surface-soft"
                    :class="[row.year === year ? 'bg-accent-wash' : '', cursor.isActive('ovr-' + row.year) && 'ring-2 ring-inset ring-accent']"
                    @click="pickYear(row.year)"
                >
                    <span class="text-left font-semibold text-ink">{{ row.year }}</span>
                    <span class="text-muted">{{ nf(row.accrual) }}</span>
                    <span class="flex flex-col items-end text-track">
                        {{ signed(row.banked) }}
                        <span v-if="Number(row.bankedPlanned) !== 0" class="text-[9px] text-behind">{{ signed(row.bankedPlanned) }} pl.</span>
                    </span>
                    <span class="flex flex-col items-end text-muted">
                        {{ signed(row.taken) }}
                        <span v-if="Number(row.takenPlanned) !== 0" class="text-[9px] text-behind">{{ signed(row.takenPlanned) }} pl.</span>
                    </span>
                    <span class="font-semibold text-ink">{{ nf(row.balance) }}</span>
                </button>
            </Card>
        </div>

        <CellEditor
            :open="editor.open"
            :x="editor.x"
            :y="editor.y"
            :start="editor.start"
            :end="editor.end"
            :existing="editor.existing"
            @save="save"
            @delete="remove"
            @close="closeEditor"
        />
    </div>
</template>

<script setup>
import { computed, ref } from 'vue';

// Year-at-a-glance wall planner (ADR-0010): 12 month-rows × 31 day-columns.
// Click a day or drag a range → `select` with the range + any existing row +
// the pointer position so the page can anchor the editor popover.
const props = defineProps({
    year: { type: Number, required: true },
    adjustments: { type: Array, default: () => [] },
    offWeekday: { type: Number, default: 5 }, // ISO 1=Mon … 7=Sun
});
const emit = defineEmits(['select']);

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const DAYS = Array.from({ length: 31 }, (_, i) => i + 1);

function iso(month, day) {
    return props.year + '-' + String(month).padStart(2, '0') + '-' + String(day).padStart(2, '0');
}
function daysIn(month) {
    return new Date(props.year, month, 0).getDate();
}
// ISO weekday (1=Mon … 7=Sun) for a grid cell.
function isoDow(month, day) {
    const d = new Date(props.year, month - 1, day).getDay();
    return d === 0 ? 7 : d;
}
function nonWorking(month, day) {
    const dow = isoDow(month, day);
    return dow >= 6 || dow === props.offWeekday;
}

// date → adjustment lookup.
const byDate = computed(() => {
    const m = {};
    for (const a of props.adjustments) m[String(a.date).slice(0, 10)] = a;
    return m;
});

// Drag state — dragging over cells extends a range; a click is start === end.
const anchor = ref(null); // { month, day }
const hover = ref(null);

function inDrag(month, day) {
    if (!anchor.value || !hover.value) return false;
    const a = new Date(props.year, anchor.value.month - 1, anchor.value.day);
    const b = new Date(props.year, hover.value.month - 1, hover.value.day);
    const c = new Date(props.year, month - 1, day);
    const [lo, hi] = a <= b ? [a, b] : [b, a];
    return c >= lo && c <= hi;
}

function down(month, day) {
    if (day > daysIn(month)) return;
    anchor.value = { month, day };
    hover.value = { month, day };
}
function over(month, day) {
    if (anchor.value && day <= daysIn(month)) hover.value = { month, day };
}
function up(event, month, day) {
    if (!anchor.value) return;
    const a = new Date(props.year, anchor.value.month - 1, anchor.value.day);
    const b = new Date(props.year, hover.value.month - 1, hover.value.day);
    const [lo, hi] = a <= b ? [a, b] : [b, a];
    const start = fmt(lo);
    const end = fmt(hi);
    anchor.value = null;
    hover.value = null;

    emit('select', {
        start,
        end,
        existing: start === end ? (byDate.value[start] ?? null) : null,
        x: event.clientX,
        y: event.clientY,
    });
}
function fmt(d) {
    return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0');
}
// Released on a gap (no cell caught it) → abandon the drag silently.
function cancelDrag() {
    anchor.value = null;
    hover.value = null;
}

// Fill/outline classes per type + status. Confirmed = solid, planned = outline.
function cellClass(month, day) {
    if (day > daysIn(month)) return 'invisible';
    const a = byDate.value[iso(month, day)];
    if (a) {
        const solid = a.status === 'confirmed';
        if (a.type === 'extra') return solid ? 'bg-track text-white' : 'bg-track-tint ring-1 ring-inset ring-track text-track';
        if (a.type === 'holiday') return solid ? 'bg-holiday text-white' : 'bg-holiday-tint ring-1 ring-inset ring-holiday text-holiday';
        if (a.type === 'sick') return solid ? 'bg-sick text-white' : 'bg-sick-tint ring-1 ring-inset ring-sick text-sick';
        return solid ? 'bg-off text-white' : 'bg-off-tint ring-1 ring-inset ring-off text-off';
    }
    if (nonWorking(month, day)) return 'bg-sunken';
    return 'bg-surface hover:bg-accent-wash';
}
// Every changed cell shows its signed hours (+8 an extra day, −8 a day off,
// −1,5 an early finish).
function cellLabel(month, day) {
    const a = byDate.value[iso(month, day)];
    if (!a) return '';
    const h = Number(a.hours);
    return (h > 0 ? '+' : '−') + String(Math.abs(h)).replace('.', ',');
}
function cellTitle(month, day) {
    const a = byDate.value[iso(month, day)];
    return a?.reason || '';
}
</script>

<template>
    <div class="select-none overflow-x-auto" @mouseleave="cancelDrag" @mouseup="cancelDrag">
        <div class="min-w-[860px]">
            <!-- day-number header -->
            <div class="grid grid-cols-[38px_repeat(31,minmax(0,1fr))] gap-[3px] pb-1.5">
                <div></div>
                <div
                    v-for="d in DAYS"
                    :key="d"
                    class="text-center font-mono text-[10px] font-medium text-faint-3"
                >
                    {{ d }}
                </div>
            </div>

            <!-- month rows -->
            <div
                v-for="(name, mi) in MONTHS"
                :key="name"
                class="mb-[3px] grid grid-cols-[38px_repeat(31,minmax(0,1fr))] items-center gap-[3px]"
            >
                <div class="font-mono text-[10.5px] font-semibold uppercase tracking-[0.06em] text-faint-2">{{ name }}</div>
                <div
                    v-for="d in DAYS"
                    :key="d"
                    class="relative flex aspect-square cursor-pointer items-center justify-center rounded-[5px] font-mono text-[9px] font-semibold tabular-nums transition-colors"
                    :class="[cellClass(mi + 1, d), inDrag(mi + 1, d) ? 'ring-2 ring-inset ring-accent' : '']"
                    :title="cellTitle(mi + 1, d)"
                    @mousedown.prevent="down(mi + 1, d)"
                    @mouseover="over(mi + 1, d)"
                    @mouseup.prevent="up($event, mi + 1, d)"
                >
                    {{ cellLabel(mi + 1, d) }}
                </div>
            </div>
        </div>
    </div>
</template>

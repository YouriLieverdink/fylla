<script setup>
import { computed } from 'vue';
import { Link } from '@inertiajs/vue3';
import Card from './Card.vue';

const props = defineProps({
    // When set, the chart region (not the footer slot) becomes a drill-down Link.
    href: { type: String, default: null },
    initials: { type: String, default: '' },
    name: { type: String, default: '' },
    meta: { type: String, default: '' },
    hours: { type: Number, default: 0 }, // delivered so far this month
    target: { type: Number, default: null }, // null = no target: burn-up only
    projected: { type: Number, default: null }, // run-rate total, null at month start
    overUnder: { type: Number, default: null }, // signed hours vs target
    series: { type: Array, default: () => [] }, // cumulative delivered hours, day 1..today
    today: { type: Number, default: 1 }, // day-of-month
    daysInMonth: { type: Number, default: 30 },
    daysLeft: { type: String, default: '' },
});

const hasTarget = computed(() => props.target !== null && props.target !== '');
const hasProjection = computed(() => props.projected !== null);
// On target = projected within ±5% of the agreed hours → track; otherwise
// (under-delivering OR over-running) → behind.
const tone = computed(() => {
    if (!hasTarget.value || !hasProjection.value) return 'track';
    return Math.abs(props.projected - props.target) <= props.target * 0.05 ? 'track' : 'behind';
});

// Plot box (matches the kit's viewBox 0 0 360 200).
const X0 = 24;
const X1 = 344;
const Y_TOP = 24;
const Y_BOTTOM = 168;

const yMax = computed(() => Math.max(props.target ?? 0, props.projected ?? 0, ...props.series, 1) * 1.15);
const xForDay = (d) => X0 + (d / props.daysInMonth) * (X1 - X0);
const yForHours = (h) => Y_BOTTOM - (h / yMax.value) * (Y_BOTTOM - Y_TOP);

// Actual: day 0 at the baseline, then one point per elapsed day.
const actualPoints = computed(() => {
    const pts = [[X0, Y_BOTTOM]];
    props.series.forEach((h, i) => pts.push([xForDay(i + 1), yForHours(h)]));
    return pts;
});
const actualLine = computed(() => actualPoints.value.map((p) => p.join(',')).join(' '));
const actualArea = computed(() => {
    const last = actualPoints.value[actualPoints.value.length - 1];
    return `M${actualPoints.value.map((p) => p.join(',')).join(' L')} L${last[0]},${Y_BOTTOM} Z`;
});
const todayPoint = computed(() => actualPoints.value[actualPoints.value.length - 1]);
const projectionLine = computed(() => {
    if (!hasProjection.value) return null;
    const end = [xForDay(props.daysInMonth), yForHours(props.projected)];
    return `${todayPoint.value.join(',')} ${end.join(',')}`;
});
const targetY = computed(() => (hasTarget.value ? yForHours(props.target) : null));

const gridLevels = computed(() =>
    [0, 0.25, 0.5, 0.75, 1].map((f) => ({
        y: Y_BOTTOM - f * (Y_BOTTOM - Y_TOP),
        label: Math.round(f * yMax.value),
    })),
);

const paceLabel = computed(() => {
    if (props.overUnder === null) return '—';
    const v = props.overUnder;
    return `${v > 0 ? '+' : v < 0 ? '−' : '±'}${Math.abs(v)}h`;
});

const avatar = { track: 'bg-track-tint text-track', behind: 'bg-behind-tint text-behind' };
const paceColor = { track: 'text-track', behind: 'text-behind' };
const LINE = '#8074cf';
</script>

<template>
    <Card radius="24px" pad="0" class="flex flex-col overflow-hidden">
      <component
        :is="href ? Link : 'div'"
        :href="href || undefined"
        class="flex flex-1 flex-col px-[30px] pb-[28px] pt-[28px]"
        :class="href && 'transition hover:bg-canvas/40'"
      >
        <div class="mb-1.5 flex items-start justify-between">
            <div class="flex items-center gap-[13px]">
                <div
                    class="flex h-10 w-10 items-center justify-center rounded-[13px] font-mono text-[15px] font-semibold"
                    :class="avatar[tone]"
                >
                    {{ initials }}
                </div>
                <div>
                    <div class="text-[16px] font-semibold tracking-[-0.01em]">{{ name }}</div>
                    <div class="mt-[3px] text-[12.5px] text-faint-2">{{ meta }}</div>
                </div>
            </div>
            <div class="flex flex-col items-end gap-1.5 font-mono text-[11px] font-medium">
                <span class="inline-flex items-center gap-1.5 text-muted">
                    <span class="inline-block h-2 w-3.5 rounded-sm bg-accent-tint-2"></span>delivered
                </span>
                <span v-if="hasProjection" class="inline-flex items-center gap-1.5 text-muted">
                    <span class="inline-block w-3.5 border-t-[2px] border-dashed" :style="{ borderColor: LINE }"></span>projection
                </span>
                <span v-if="hasTarget" class="inline-flex items-center gap-1.5 text-faint">
                    <span class="inline-block w-3.5 border-t-[1.5px] border-dashed border-behind"></span>target
                </span>
            </div>
        </div>
        <div class="mt-2 flex flex-1 items-center">
            <svg viewBox="0 0 360 200" width="100%" class="block">
                <g stroke="#f0ece4" stroke-width="1">
                    <line v-for="g in gridLevels" :key="`grid-${g.y}`" x1="24" :y1="g.y" x2="344" :y2="g.y" />
                </g>
                <g font-family="var(--font-mono)" font-size="8.5" fill="#c2bdb1" text-anchor="end">
                    <text v-for="g in gridLevels" :key="`lbl-${g.y}`" x="19" :y="g.y + 3">{{ g.label }}</text>
                </g>

                <!-- target reference line -->
                <line
                    v-if="targetY !== null"
                    x1="24"
                    :y1="targetY"
                    x2="344"
                    :y2="targetY"
                    stroke="#b18749"
                    stroke-width="1.5"
                    stroke-dasharray="4 4"
                />

                <!-- actual delivered -->
                <path :d="actualArea" fill="#c9c2ee" opacity=".28" />
                <polyline
                    :points="actualLine"
                    fill="none"
                    :stroke="LINE"
                    stroke-width="2.25"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                />

                <!-- projection today → month-end -->
                <polyline
                    v-if="projectionLine"
                    :points="projectionLine"
                    fill="none"
                    :stroke="LINE"
                    stroke-width="2"
                    stroke-dasharray="4 4"
                    stroke-linecap="round"
                />
                <circle :cx="todayPoint[0]" :cy="todayPoint[1]" r="4" :fill="LINE" stroke="#fff" stroke-width="2" />
                <line :x1="todayPoint[0]" y1="20" :x2="todayPoint[0]" y2="176" stroke="#d8d3c8" stroke-width="1" stroke-dasharray="2 3" />

                <g font-family="var(--font-mono)" font-size="8.5" fill="#a8a498">
                    <text x="24" y="190">1</text>
                    <text :x="todayPoint[0]" y="190" text-anchor="middle" fill="#6d6a63">today</text>
                    <text x="344" y="190" text-anchor="end">{{ daysInMonth }}</text>
                </g>
            </svg>
        </div>
        <div class="mt-3.5 flex gap-7 border-t border-divider pt-4">
            <div>
                <div class="mb-[5px] font-mono text-[10.5px] text-faint-2">DELIVERED</div>
                <div class="font-mono text-[17px] font-semibold tabular-nums">
                    {{ hours }}<span class="text-[13px] text-faint-4">{{ hasTarget ? ` / ${target}h` : 'h' }}</span>
                </div>
            </div>
            <div>
                <div class="mb-[5px] font-mono text-[10.5px] text-faint-2">PACE</div>
                <div class="font-mono text-[17px] font-semibold tabular-nums" :class="paceColor[tone]">{{ paceLabel }}</div>
            </div>
            <div>
                <div class="mb-[5px] font-mono text-[10.5px] text-faint-2">PROJECTED</div>
                <div class="font-mono text-[17px] font-semibold tabular-nums">
                    {{ hasProjection ? `${projected}h` : daysLeft }}
                </div>
            </div>
        </div>
      </component>
      <slot name="footer" />
    </Card>
</template>

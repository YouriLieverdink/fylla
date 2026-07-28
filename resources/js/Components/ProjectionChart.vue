<script setup>
import { computed } from 'vue';

const props = defineProps({
    history: { type: Array, default: () => [] }, // [{ label, value }] oldest → newest
    projection: { type: Array, default: () => [] }, // separate pace-held continuation
    floor: { type: Number, default: 73 },
    target: { type: Number, default: 75 },
});

const X0 = 20;
const X1 = 344;
const TOP = 24;
const BOTTOM = 118;

const pointCount = computed(() => props.history.length + props.projection.length);
const xFor = (i) => (pointCount.value <= 1 ? X0 : X0 + (i / (pointCount.value - 1)) * (X1 - X0));
const domain = computed(() => {
    const values = [...props.history, ...props.projection]
        .map((point) => point.value)
        .filter((value) => value != null);
    const lo = Math.max(0, Math.min(...values, props.floor) - 5);
    const hi = Math.max(...values, props.target) + 5;

    return { lo, hi: hi > lo ? hi : lo + 1 };
});
const yFor = (value) => {
    const { lo, hi } = domain.value;
    return BOTTOM - ((value - lo) / (hi - lo)) * (BOTTOM - TOP);
};

const runs = (coords) => {
    const segments = [];
    let current = [];
    for (const coord of coords) {
        if (coord) current.push(coord);
        else if (current.length) {
            segments.push(current);
            current = [];
        }
    }
    if (current.length) segments.push(current);

    return segments;
};
const lineOf = (coords) => runs(coords)
    .map((segment) => `M ${segment.map(([x, y]) => `${x.toFixed(1)} ${y.toFixed(1)}`).join(' L ')}`)
    .join(' ');

const historyCoords = computed(() => props.history.map((point, index) =>
    point.value == null ? null : [xFor(index), yFor(point.value)],
));
const forwardCoords = computed(() => props.projection.map((point, index) =>
    point.value == null ? null : [xFor(props.history.length + index), yFor(point.value)],
));
const historyPath = computed(() => lineOf(historyCoords.value));
const projectionPath = computed(() => {
    const latest = historyCoords.value.at(-1);
    const join = latest && forwardCoords.value[0] ? [latest] : [];

    return lineOf([...join, ...forwardCoords.value]);
});
const nullWeeks = computed(() => [
    ...historyCoords.value.map((coord, index) => coord === null ? xFor(index) : null),
    ...forwardCoords.value.map((coord, index) => coord === null ? xFor(props.history.length + index) : null),
].filter((x) => x !== null));

const floorY = computed(() => yFor(props.floor));
const targetY = computed(() => yFor(props.target));
const bandY = computed(() => Math.min(floorY.value, targetY.value));
const bandHeight = computed(() => Math.abs(floorY.value - targetY.value));
</script>

<template>
    <svg viewBox="0 0 360 150" width="100%" class="block" aria-label="Utilization history and projection">
        <rect
            data-band
            :x="X0"
            :y="bandY"
            :width="X1 - X0"
            :height="bandHeight"
            fill="#b18749"
            fill-opacity=".12"
        />
        <line
            data-floor
            :x1="X0"
            :y1="floorY"
            :x2="X1"
            :y2="floorY"
            stroke="#b18749"
            stroke-width="1.25"
            stroke-dasharray="3 4"
        />
        <text
            data-floor-label
            :x="X1"
            :y="floorY + 11"
            text-anchor="end"
            font-family="var(--font-mono)"
            font-size="9"
            fill="#9a7139"
        >{{ floor }}% floor</text>

        <path
            v-if="historyPath"
            data-series="history"
            :d="historyPath"
            fill="none"
            stroke="#6c5fc9"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
        />
        <circle
            v-for="([x, y], index) in historyCoords.filter(Boolean)"
            :key="`history-${index}`"
            data-history-point
            :cx="x"
            :cy="y"
            r="2.5"
            fill="#6c5fc9"
        />
        <path
            v-if="projectionPath"
            data-series="projection"
            :d="projectionPath"
            fill="none"
            stroke="#6c5fc9"
            stroke-width="2"
            stroke-dasharray="5 5"
            stroke-linecap="round"
            stroke-linejoin="round"
        />
        <circle
            v-for="([x, y], index) in forwardCoords.filter(Boolean)"
            :key="`forward-${index}`"
            data-forward-point
            :cx="x"
            :cy="y"
            r="2.5"
            fill="#6c5fc9"
        />
        <circle
            v-for="(x, index) in nullWeeks"
            :key="`null-${index}`"
            data-null-week
            :cx="x"
            :cy="BOTTOM"
            r="2.5"
            fill="none"
            stroke="#a8a498"
            stroke-width="1.25"
        />

        <text :x="X0" y="140" font-family="var(--font-mono)" font-size="9" fill="#a8a498">history</text>
        <text :x="X1" y="140" text-anchor="end" font-family="var(--font-mono)" font-size="9" fill="#a8a498">+{{ projection.length }} weeks</text>
    </svg>
</template>

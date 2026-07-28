<script setup>
import { computed, ref } from 'vue';

const props = defineProps({
    history: { type: Array, default: () => [] }, // [{ label, value }] oldest → newest, ending at the last *complete* week
    projection: { type: Array, default: () => [] }, // separate pace-held continuation
    floor: { type: Number, default: 73 },
    target: { type: Number, default: 75 },
});

const X0 = 20;
const X1 = 344;
const TOP = 24;
const BOTTOM = 118;

const points = computed(() => [
    ...props.history.map((point) => ({ ...point, series: 'history' })),
    ...props.projection.map((point) => ({ ...point, series: 'projection' })),
]);
const pointCount = computed(() => points.value.length);
const xFor = (i) => (pointCount.value <= 1 ? X0 : X0 + (i / (pointCount.value - 1)) * (X1 - X0));
const domain = computed(() => {
    const values = points.value.map((point) => point.value).filter((value) => value != null);
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
const coords = computed(() => [...historyCoords.value, ...forwardCoords.value]);
const historyPath = computed(() => lineOf(historyCoords.value));
const projectionPath = computed(() => {
    const latest = historyCoords.value.at(-1);
    const join = latest && forwardCoords.value[0] ? [latest] : [];

    return lineOf([...join, ...forwardCoords.value]);
});
const nullWeeks = computed(() => coords.value
    .map((coord, index) => coord === null ? xFor(index) : null)
    .filter((x) => x !== null));

const floorY = computed(() => yFor(props.floor));
const targetY = computed(() => yFor(props.target));
const bandY = computed(() => Math.min(floorY.value, targetY.value));
const bandHeight = computed(() => Math.abs(floorY.value - targetY.value));
const bandLabelY = computed(() => Math.max(bandY.value - 6, 10));
// The current week is projection[0], not the last history point, so "now" sits
// on the boundary between the two series.
const currentX = computed(() => props.history.length
    ? xFor(props.history.length - (props.projection.length ? 0.5 : 1))
    : null);

// Match the Worklist trend chart: each week owns a full-height hover band, with
// a guide line and tooltip. This is easier to hit than a tiny point.
const hover = ref(null);
const bandWidth = computed(() => (pointCount.value <= 1 ? X1 - X0 : (X1 - X0) / (pointCount.value - 1)));
const tip = computed(() => {
    if (hover.value === null) return null;
    const point = points.value[hover.value];
    const coord = coords.value[hover.value];
    const x = coord ? coord[0] : xFor(hover.value);
    const y = coord ? coord[1] : BOTTOM;
    const lines = coord
        ? [point.label, `${point.value}% ${point.series === 'projection' ? 'projected' : 'utilization'}`]
        : [point.label, 'no utilization data'];
    if (point.off) lines.push('week off');
    const width = Math.max(...lines.map((line) => line.length)) * 6 + 14;
    const height = lines.length * 13 + 7;
    const boxX = Math.min(Math.max(x - width / 2, 4), 356 - width);
    const boxY = Math.max(y - 12 - height, 4);

    return { x, y, lines, boxX, boxY, width, height, gap: !coord };
});
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
        <line
            v-if="currentX !== null"
            data-current-week
            :x1="currentX"
            :y1="TOP"
            :x2="currentX"
            :y2="BOTTOM"
            stroke="#8f8a80"
            stroke-width="1"
            opacity=".55"
        />
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
        <template v-for="(coord, index) in historyCoords" :key="`history-${index}`">
            <circle
                v-if="coord"
                data-history-point
                :cx="coord[0]"
                :cy="coord[1]"
                r="2.5"
                :fill="history[index].off ? '#fff' : '#6c5fc9'"
                stroke="#6c5fc9"
                :stroke-width="history[index].off ? 1.5 : 0"
            />
        </template>
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
        <template v-for="(coord, index) in forwardCoords" :key="`forward-${index}`">
            <circle
                v-if="coord"
                data-forward-point
                :cx="coord[0]"
                :cy="coord[1]"
                r="2.5"
                :fill="projection[index].off ? '#fff' : '#6c5fc9'"
                stroke="#6c5fc9"
                :stroke-width="projection[index].off ? 1.5 : 0"
            />
        </template>
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

        <text
            data-floor-label
            :x="X1"
            :y="bandLabelY"
            text-anchor="end"
            font-family="var(--font-mono)"
            font-size="9"
            fill="#9a7139"
            stroke="#fff"
            stroke-width="3"
            paint-order="stroke"
        >{{ floor }}% floor</text>
        <text
            v-if="currentX !== null"
            :x="currentX - 4"
            :y="bandLabelY"
            text-anchor="end"
            font-family="var(--font-mono)"
            font-size="8"
            fill="#8f8a80"
            stroke="#fff"
            stroke-width="3"
            paint-order="stroke"
        >this week</text>

        <text :x="X0" y="140" font-family="var(--font-mono)" font-size="9" fill="#a8a498">{{ history.length }} weeks history</text>
        <text :x="X1" y="140" text-anchor="end" font-family="var(--font-mono)" font-size="9" fill="#a8a498">{{ projection.length }} weeks projected</text>

        <g v-if="tip" data-tooltip pointer-events="none">
            <line :x1="tip.x" :y1="TOP" :x2="tip.x" :y2="BOTTOM" stroke="#6c5fc9" stroke-width="1" stroke-dasharray="2 3" opacity=".4" />
            <circle :cx="tip.x" :cy="tip.y" r="4" :fill="tip.gap ? 'none' : '#6c5fc9'" :stroke="tip.gap ? '#a8a498' : '#fff'" stroke-width="2" />
            <rect :x="tip.boxX" :y="tip.boxY" :width="tip.width" :height="tip.height" rx="6" fill="#2b2a27" opacity=".88" />
            <text
                v-for="(line, index) in tip.lines"
                :key="index"
                :x="tip.boxX + 7"
                :y="tip.boxY + 15 + index * 13"
                font-family="var(--font-mono)"
                font-size="10"
                :font-weight="index === 0 ? 600 : 400"
                fill="#fff"
            >{{ line }}</text>
        </g>
        <rect
            v-for="(point, index) in points"
            :key="`hover-${index}`"
            data-hover-band
            :x="xFor(index) - bandWidth / 2"
            :y="TOP"
            :width="bandWidth"
            :height="BOTTOM - TOP"
            fill="transparent"
            tabindex="0"
            :aria-label="point.value === null ? `${point.label}: no utilization data` : `${point.label}: ${point.value}% ${point.series === 'projection' ? 'projected utilization' : 'utilization'}${point.off ? ', week off' : ''}`"
            @mouseenter="hover = index"
            @mouseleave="hover = null"
            @focus="hover = index"
            @blur="hover = null"
        />
    </svg>
</template>

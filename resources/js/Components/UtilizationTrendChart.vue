<script setup>
import { computed, ref } from 'vue';
import Card from './Card.vue';

const props = defineProps({
    points: { type: Array, default: () => [] }, // [{ label, value }] oldest → newest
    target: { type: Number, default: 75 },
    weeks: { type: Number, default: 13 },
});

// Plot geometry (matches the kit: viewBox 360×150, fill baseline y=118).
const X0 = 20;
const X1 = 344;
const TOP = 30;
const BOTTOM = 118;

// Domain padded around the data + target so the line and target both sit inside.
const domain = computed(() => {
    const shares = props.points.map((p) => p.billableShare).filter((v) => v != null);
    const values = props.points.map((p) => p.value).filter((v) => v != null);
    const vals = [...values, ...shares, props.target];
    const lo = Math.max(0, Math.min(...vals) - 8);
    const hi = Math.max(...vals) + 8;
    return { lo, hi: hi > lo ? hi : lo + 1 };
});

const yFor = (v) => {
    const { lo, hi } = domain.value;
    return BOTTOM - ((v - lo) / (hi - lo)) * (BOTTOM - TOP);
};
const xFor = (i, n) => (n <= 1 ? X0 : X0 + (i / (n - 1)) * (X1 - X0));

// Split a coord array (nulls = gaps) into runs of consecutive plotted points.
const runs = (cs) => {
    const segs = [];
    let cur = [];
    for (const c of cs) {
        if (c) cur.push(c);
        else if (cur.length) (segs.push(cur), (cur = []));
    }
    if (cur.length) segs.push(cur);
    return segs;
};
const lineOf = (cs) => runs(cs).map((seg) => 'M ' + seg.map(([x, y]) => `${x.toFixed(1)} ${y.toFixed(1)}`).join(' L ')).join(' ');

// null value = a zero-capacity week (all time off): a gap, not a plotted point.
const coords = computed(() =>
    props.points.map((p, i) => (p.value == null ? null : [xFor(i, props.points.length), yFor(p.value)])),
);
const linePath = computed(() => lineOf(coords.value));
const area = computed(() =>
    runs(coords.value)
        .map((seg) => {
            const pts = seg.map(([x, y]) => `L ${x.toFixed(1)},${y.toFixed(1)}`).join(' ');
            return `M ${seg[0][0].toFixed(1)} ${BOTTOM} ${pts} L ${seg[seg.length - 1][0].toFixed(1)} ${BOTTOM} Z`;
        })
        .join(' '),
);
// x positions of gap weeks, for the baseline "off" markers.
const gaps = computed(() =>
    props.points.map((p, i) => (p.value == null ? xFor(i, props.points.length) : null)).filter((x) => x != null),
);
const last = computed(() => {
    for (let i = coords.value.length - 1; i >= 0; i--) if (coords.value[i]) return coords.value[i];
    return null;
});
// Isolated plotted points (a gap on both sides): a 1-point path stroke renders
// nothing, so mark them explicitly. Applies to both series.
const dotsOf = (cs) => runs(cs).filter((seg) => seg.length === 1).map((seg) => seg[0]);
const utilDots = computed(() => dotsOf(coords.value));

// Billable share line (billable ÷ worked). Breaks at the same gaps as the
// utilization line (and any week with no worked hours), rather than bridging.
const shareCoords = computed(() =>
    props.points.map((p, i) => (p.billableShare == null ? null : [xFor(i, props.points.length), yFor(p.billableShare)])),
);
const sharePath = computed(() => lineOf(shareCoords.value));
const shareDots = computed(() => dotsOf(shareCoords.value));
const targetY = computed(() => yFor(props.target));

const hover = ref(null); // active point index
const bandW = computed(() => (props.points.length <= 1 ? X1 - X0 : (X1 - X0) / (props.points.length - 1)));
const tip = computed(() => {
    if (hover.value == null) return null;
    const p = props.points[hover.value];
    const c = coords.value[hover.value];
    const x = c ? c[0] : xFor(hover.value, props.points.length);
    const y = c ? c[1] : BOTTOM;
    const lines = c ? [p.label, `${p.value}% utilization`] : [p.label, 'week off'];
    if (c && p.billableShare != null) lines.push(`${p.billableShare}% billable`);
    // Size the box off the longest line (~6px/char mono) and clamp inside 360w.
    const w = Math.max(...lines.map((l) => l.length)) * 6 + 14;
    const h = lines.length * 13 + 7;
    const bx = Math.min(Math.max(x - w / 2, 4), 356 - w);
    const by = Math.max(y - 12 - h, 4);
    return { x, y, lines, bx, by, w, h, gap: !c };
});
</script>

<template>
    <Card radius="24px" pad="28px 30px" class="flex flex-col">
        <div class="mb-1.5 flex items-start justify-between">
            <div>
                <div class="text-[16px] font-semibold tracking-[-0.01em]">Utilization trend</div>
                <div class="mt-[3px] text-[12.5px] text-faint-2">Billable % · rolling {{ weeks }} weeks</div>
            </div>
            <div class="flex items-center gap-4 font-mono text-[11px] font-medium">
                <span class="inline-flex items-center gap-1.5 text-muted">
                    <span class="inline-block h-0.5 w-3.5 rounded-sm bg-accent"></span>utilization
                </span>
                <span class="inline-flex items-center gap-1.5 text-muted">
                    <span class="inline-block h-0.5 w-3.5 rounded-sm bg-track"></span>billable share
                </span>
                <span class="inline-flex items-center gap-1.5 text-faint">
                    <span class="inline-block w-3.5 border-t-[1.5px] border-dashed border-behind"></span>{{ target }}% target
                </span>
            </div>
        </div>
        <div class="mt-2 flex flex-1 items-center">
            <svg v-if="points.some((p) => p.value != null)" viewBox="0 0 360 150" width="100%" class="block">
                <line :x1="X0" :y1="targetY" :x2="X1" :y2="targetY" stroke="#b18749" stroke-width="1.25" stroke-dasharray="3 4" opacity=".85" />
                <text :x="X1" :y="targetY - 5" text-anchor="end" font-family="var(--font-mono)" font-size="9" fill="#b18749">{{ target }}%</text>
                <path :d="area" fill="url(#utilFill)" />
                <defs>
                    <linearGradient id="utilFill" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0" stop-color="#6c5fc9" stop-opacity=".16" />
                        <stop offset="1" stop-color="#6c5fc9" stop-opacity="0" />
                    </linearGradient>
                </defs>
                <path v-if="sharePath" :d="sharePath" fill="none" stroke="#5c8a6f" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" />
                <circle v-for="([dx, dy], i) in shareDots" :key="'share' + i" :cx="dx" :cy="dy" r="2.5" fill="#5c8a6f" />
                <path :d="linePath" fill="none" stroke="#6c5fc9" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
                <circle v-for="([dx, dy], i) in utilDots" :key="'util' + i" :cx="dx" :cy="dy" r="2.75" fill="#6c5fc9" />
                <circle v-for="(gx, i) in gaps" :key="'gap' + i" :cx="gx" :cy="BOTTOM" r="2.5" fill="none" stroke="#a8a498" stroke-width="1.25" />
                <circle v-if="last" :cx="last[0]" :cy="last[1]" r="4" fill="#6c5fc9" stroke="#fff" stroke-width="2" />
                <text :x="X0" y="140" font-family="var(--font-mono)" font-size="9" fill="#a8a498">{{ weeks }}w ago</text>
                <text :x="X1" y="140" text-anchor="end" font-family="var(--font-mono)" font-size="9" fill="#a8a498">now</text>

                <!-- hover layer: a full-height band per point, tooltip on the active one -->
                <g v-if="tip">
                    <line :x1="tip.x" :y1="TOP" :x2="tip.x" :y2="BOTTOM" stroke="#6c5fc9" stroke-width="1" stroke-dasharray="2 3" opacity=".4" />
                    <circle :cx="tip.x" :cy="tip.y" r="4" :fill="tip.gap ? 'none' : '#6c5fc9'" :stroke="tip.gap ? '#a8a498' : '#fff'" stroke-width="2" />
                    <rect :x="tip.bx" :y="tip.by" :width="tip.w" :height="tip.h" rx="6" fill="#2b2a27" opacity="0.88" />
                    <text
                        v-for="(ln, i) in tip.lines"
                        :key="i"
                        :x="tip.bx + 7"
                        :y="tip.by + 15 + i * 13"
                        font-family="var(--font-mono)"
                        font-size="10"
                        :font-weight="i === 0 ? 600 : 400"
                        fill="#fff"
                        >{{ ln }}</text
                    >
                </g>
                <rect
                    v-for="(p, i) in points"
                    :key="i"
                    :x="xFor(i, points.length) - bandW / 2"
                    :y="TOP"
                    :width="bandW"
                    :height="BOTTOM - TOP"
                    fill="transparent"
                    @mouseenter="hover = i"
                    @mouseleave="hover = null"
                />
            </svg>
            <div v-else class="w-full py-10 text-center text-[12.5px] text-faint-2">No billable weeks in this window yet.</div>
        </div>
    </Card>
</template>

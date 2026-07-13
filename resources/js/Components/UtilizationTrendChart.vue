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
    const vals = [...props.points.map((p) => p.value), props.target];
    const lo = Math.max(0, Math.min(...vals) - 8);
    const hi = Math.max(...vals) + 8;
    return { lo, hi: hi > lo ? hi : lo + 1 };
});

const yFor = (v) => {
    const { lo, hi } = domain.value;
    return BOTTOM - ((v - lo) / (hi - lo)) * (BOTTOM - TOP);
};
const xFor = (i, n) => (n <= 1 ? X0 : X0 + (i / (n - 1)) * (X1 - X0));

const coords = computed(() => props.points.map((p, i) => [xFor(i, props.points.length), yFor(p.value)]));
const polyline = computed(() => coords.value.map(([x, y]) => `${x.toFixed(1)},${y.toFixed(1)}`).join(' '));
const area = computed(() => {
    if (!coords.value.length) return '';
    const pts = coords.value.map(([x, y]) => `L ${x.toFixed(1)},${y.toFixed(1)}`).join(' ');
    const first = coords.value[0];
    const last = coords.value[coords.value.length - 1];
    return `M ${first[0].toFixed(1)} ${BOTTOM} ${pts} L ${last[0].toFixed(1)} ${BOTTOM} Z`;
});
const last = computed(() => coords.value[coords.value.length - 1] ?? null);
const targetY = computed(() => yFor(props.target));

const hover = ref(null); // active point index
const bandW = computed(() => (props.points.length <= 1 ? X1 - X0 : (X1 - X0) / (props.points.length - 1)));
const tip = computed(() => {
    if (hover.value == null) return null;
    const [x, y] = coords.value[hover.value];
    const p = props.points[hover.value];
    // Anchor the label so it stays inside the viewBox at both ends.
    const anchor = x < 60 ? 'start' : x > X1 - 40 ? 'end' : 'middle';
    return { x, y, anchor, text: `${p.label} · ${p.value}%` };
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
                    <span class="inline-block h-0.5 w-3.5 rounded-sm bg-accent"></span>billable
                </span>
                <span class="inline-flex items-center gap-1.5 text-faint">
                    <span class="inline-block w-3.5 border-t-[1.5px] border-dashed border-behind"></span>{{ target }}% target
                </span>
            </div>
        </div>
        <div class="mt-2 flex flex-1 items-center">
            <svg v-if="points.length" viewBox="0 0 360 150" width="100%" class="block">
                <line :x1="X0" :y1="targetY" :x2="X1" :y2="targetY" stroke="#b18749" stroke-width="1.25" stroke-dasharray="3 4" opacity=".85" />
                <text :x="X1" :y="targetY - 5" text-anchor="end" font-family="var(--font-mono)" font-size="9" fill="#b18749">{{ target }}%</text>
                <path :d="area" fill="url(#utilFill)" />
                <defs>
                    <linearGradient id="utilFill" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0" stop-color="#6c5fc9" stop-opacity=".16" />
                        <stop offset="1" stop-color="#6c5fc9" stop-opacity="0" />
                    </linearGradient>
                </defs>
                <polyline :points="polyline" fill="none" stroke="#6c5fc9" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
                <circle v-if="last" :cx="last[0]" :cy="last[1]" r="4" fill="#6c5fc9" stroke="#fff" stroke-width="2" />
                <text :x="X0" y="140" font-family="var(--font-mono)" font-size="9" fill="#a8a498">{{ weeks }}w ago</text>
                <text :x="X1" y="140" text-anchor="end" font-family="var(--font-mono)" font-size="9" fill="#a8a498">now</text>

                <!-- hover layer: a full-height band per point, tooltip on the active one -->
                <g v-if="tip">
                    <line :x1="tip.x" :y1="TOP" :x2="tip.x" :y2="BOTTOM" stroke="#6c5fc9" stroke-width="1" stroke-dasharray="2 3" opacity=".4" />
                    <circle :cx="tip.x" :cy="tip.y" r="4" fill="#6c5fc9" stroke="#fff" stroke-width="2" />
                    <text
                        :x="tip.x"
                        :y="tip.y - 10"
                        :text-anchor="tip.anchor"
                        font-family="var(--font-mono)"
                        font-size="10"
                        font-weight="600"
                        fill="#6c5fc9"
                        >{{ tip.text }}</text
                    >
                </g>
                <rect
                    v-for="(c, i) in coords"
                    :key="i"
                    :x="c[0] - bandW / 2"
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

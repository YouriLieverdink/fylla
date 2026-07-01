<script setup>
import Card from './Card.vue';

defineProps({
    value: { type: String, default: '+38%' },
    subtitle: { type: String, default: 'Rolling · last 40 issues' },
    biasPos: { type: Number, default: 69 }, // marker position on the under→over scale (%)
    medianEst: { type: String, default: '4.0h' },
    medianActual: { type: String, default: '5.5h' },
    withinPct: { type: String, default: '31%' },
});
</script>

<template>
    <Card radius="24px" pad="26px 28px">
        <div class="mb-[5px] font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">Estimate bias</div>
        <div class="mb-[22px] text-[12.5px] text-faint-2">{{ subtitle }}</div>

        <div class="mb-1.5 flex items-baseline gap-2">
            <span class="font-mono text-[40px] font-semibold leading-[0.9] tabular-nums tracking-[-0.03em] text-behind">{{
                value
            }}</span>
        </div>
        <div class="mb-[22px] text-[13.5px] leading-[1.5] text-muted">
            You tend to <strong class="font-semibold">underestimate by ~40%</strong>. Not a problem — just pad new
            estimates a little.
        </div>

        <div
            class="relative mb-2 h-1.5 rounded-full"
            style="background: linear-gradient(90deg, #e6e2d9, #edeae3, #e6e2d9)"
        >
            <div class="absolute -top-[3px] left-1/2 h-3 w-px -translate-x-1/2 bg-[#cbc6ba]"></div>
            <div
                class="absolute -top-1 h-3.5 w-3.5 -translate-x-1/2 rounded-full border-[2.5px] border-white bg-behind shadow-[0_2px_6px_rgba(42,41,38,0.15)]"
                :style="{ left: biasPos + '%' }"
            ></div>
        </div>
        <div class="flex justify-between font-mono text-[10px] font-medium text-faint-3">
            <span>under-run</span>
            <span>on point</span>
            <span>over-run</span>
        </div>

        <div class="mt-6 flex flex-col gap-3.5 border-t border-divider pt-5">
            <div class="flex items-center justify-between">
                <span class="text-[13px] text-muted">Median est.</span>
                <span class="font-mono text-[13px] font-medium tabular-nums">{{ medianEst }}</span>
            </div>
            <div class="flex items-center justify-between">
                <span class="text-[13px] text-muted">Median actual</span>
                <span class="font-mono text-[13px] font-medium tabular-nums">{{ medianActual }}</span>
            </div>
            <div class="flex items-center justify-between">
                <span class="text-[13px] text-muted">Within ±15%</span>
                <span class="font-mono text-[13px] font-medium tabular-nums text-track">{{ withinPct }}</span>
            </div>
        </div>
    </Card>
</template>

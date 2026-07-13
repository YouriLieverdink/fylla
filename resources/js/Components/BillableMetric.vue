<script setup>
import { computed } from 'vue';
import Card from './Card.vue';
import Chip from './Chip.vue';

const props = defineProps({
    value: { type: Number, default: null }, // null → no capacity in window
    status: { type: String, default: 'on track' },
    delta: { type: String, default: null },
    deltaCaption: { type: String, default: '' },
    target: { type: Number, default: 75 },
    note: { type: String, default: '' },
    week: { type: Object, default: () => ({ value: null, billableHours: 0, capacityHours: 0 }) },
});

const hasValue = computed(() => props.value != null);
const barWidth = (v) => Math.min(100, Math.max(0, v ?? 0)) + '%';
</script>

<template>
    <Card radius="24px" pad="32px 34px" accent class="relative overflow-hidden">
        <div
            class="pointer-events-none absolute -right-10 -top-10 h-[150px] w-[150px] rounded-full"
            style="background: radial-gradient(circle, rgba(108, 95, 201, 0.1), transparent 70%)"
        ></div>

        <div class="mb-5 flex items-start justify-between">
            <div>
                <div class="mb-1.5 font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">
                    Billable utilization
                </div>
                <div class="text-[12.5px] text-faint-2">{{ deltaCaption }}</div>
            </div>
            <Chip tone="accent" dot>{{ status }}</Chip>
        </div>

        <div class="my-0.5 flex items-end gap-1">
            <span
                class="font-mono font-semibold leading-[0.86] tabular-nums tracking-[-0.04em] text-accent"
                style="font-size: 82px"
                >{{ hasValue ? value : '—' }}</span
            >
            <span v-if="hasValue" class="mb-2 font-mono text-[30px] font-medium text-accent-tint-2">%</span>
        </div>

        <div v-if="delta" class="mt-4 flex items-center gap-3.5">
            <span class="inline-flex items-center gap-1.5 font-mono text-[13px] font-medium text-track">
                <svg width="13" height="13" viewBox="0 0 12 12" fill="none">
                    <path
                        d="M6 9.5V2.5M6 2.5L3 5.5M6 2.5L9 5.5"
                        stroke="currentColor"
                        stroke-width="1.5"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                    />
                </svg>
                {{ delta }}
            </span>
            <span class="text-[13px] text-faint">{{ deltaCaption }}</span>
        </div>

        <div class="mt-7">
            <div class="mb-[9px] flex justify-between font-mono text-[11px] font-medium text-faint">
                <span>0%</span>
                <span class="text-accent">target {{ target }}%</span>
                <span>100%</span>
            </div>
            <div class="relative h-2 overflow-hidden rounded-full bg-sunken">
                <div class="absolute inset-y-0 left-0 rounded-full bg-accent" :style="{ width: barWidth(value) }"></div>
            </div>
            <div class="relative h-0">
                <div
                    class="absolute -top-3.5 h-4 w-0.5 -translate-x-1/2 rounded-sm bg-accent-deep"
                    :style="{ left: target + '%' }"
                ></div>
            </div>
        </div>

        <!-- This week gauge: the operational number alongside the cumulative headline -->
        <div class="mt-6 border-t border-divider-soft pt-4">
            <div class="mb-[7px] flex items-baseline justify-between">
                <span class="font-mono text-[11px] font-semibold uppercase tracking-[0.1em] text-faint">This week</span>
                <span class="font-mono text-[13px] font-medium tabular-nums text-muted">
                    <template v-if="week.value != null"
                        >{{ week.value }}% · {{ week.billableHours }}/{{ week.capacityHours }}h</template
                    >
                    <template v-else>—</template>
                </span>
            </div>
            <div class="relative h-1.5 overflow-hidden rounded-full bg-sunken">
                <div class="absolute inset-y-0 left-0 rounded-full bg-accent-tint-2" :style="{ width: barWidth(week.value) }"></div>
            </div>
        </div>

        <p v-if="note" class="mt-[18px] text-[12.5px] leading-[1.55] text-faint-2">{{ note }}</p>
    </Card>
</template>

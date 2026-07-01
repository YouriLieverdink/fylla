<script setup>
import Card from './Card.vue';
// ponytail: SVG geometry is the kit's demo data, hardcoded. Wire real
// rolling-13-week points via props when a screen consumes this.
defineProps({ demo: { type: Boolean, default: false } });
</script>

<template>
    <Card radius="24px" pad="28px 30px" class="flex flex-col">
        <div class="mb-1.5 flex items-start justify-between">
            <div>
                <div class="flex items-center gap-2.5">
                    <div class="text-[16px] font-semibold tracking-[-0.01em]">Utilization trend</div>
                    <span
                        v-if="demo"
                        class="inline-flex items-center rounded-full bg-divider px-2.5 py-[5px] font-mono text-[9.5px] font-semibold uppercase tracking-[0.1em] text-faint"
                        >demo data</span
                    >
                </div>
                <div class="mt-[3px] text-[12.5px] text-faint-2">Billable % · rolling 13 weeks</div>
            </div>
            <div class="flex items-center gap-4 font-mono text-[11px] font-medium">
                <span class="inline-flex items-center gap-1.5 text-muted">
                    <span class="inline-block h-0.5 w-3.5 rounded-sm bg-accent"></span>billable
                </span>
                <span class="inline-flex items-center gap-1.5 text-faint">
                    <span class="inline-block w-3.5 border-t-[1.5px] border-dashed border-behind"></span>75% target
                </span>
            </div>
        </div>
        <div class="mt-2 flex flex-1 items-center">
            <svg viewBox="0 0 360 150" width="100%" class="block">
                <line x1="20" y1="44.3" x2="344" y2="44.3" stroke="#b18749" stroke-width="1.25" stroke-dasharray="3 4" opacity=".85" />
                <text x="344" y="39" text-anchor="end" font-family="var(--font-mono)" font-size="9" fill="#b18749">75%</text>
                <path
                    d="M 20 118 L 20.0,75.5 L 47.0,87.0 L 74.0,65.7 L 101.0,81.3 L 128.0,60.0 L 155.0,47.8 L 182.0,61.6 L 209.0,54.3 L 236.0,42.9 L 263.0,51.9 L 290.0,60.8 L 317.0,46.1 L 344.0,54.3 L 344 118 Z"
                    fill="url(#utilFill)"
                />
                <defs>
                    <linearGradient id="utilFill" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0" stop-color="#6c5fc9" stop-opacity=".16" />
                        <stop offset="1" stop-color="#6c5fc9" stop-opacity="0" />
                    </linearGradient>
                </defs>
                <polyline
                    points="20.0,75.5 47.0,87.0 74.0,65.7 101.0,81.3 128.0,60.0 155.0,47.8 182.0,61.6 209.0,54.3 236.0,42.9 263.0,51.9 290.0,60.8 317.0,46.1 344.0,54.3"
                    fill="none"
                    stroke="#6c5fc9"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                />
                <circle cx="344" cy="54.3" r="4" fill="#6c5fc9" stroke="#fff" stroke-width="2" />
                <text x="20" y="140" font-family="var(--font-mono)" font-size="9" fill="#a8a498">13w ago</text>
                <text x="344" y="140" text-anchor="end" font-family="var(--font-mono)" font-size="9" fill="#a8a498">now</text>
            </svg>
        </div>
    </Card>
</template>

<script setup>
import Card from './Card.vue';
// ponytail: SVG geometry is the kit's demo data, hardcoded. Wire real
// committed/actual/ideal series via props when a screen consumes this.

defineProps({
    subtitle: { type: String, default: 'Sprint 24 · Meridian redesign · day 8 of 10' },
    delivered: { type: String, default: '90' },
    deliveredOf: { type: String, default: ' / 108 pts' },
    pace: { type: String, default: '+4 pts' },
    projected: { type: String, default: 'on time' },
});
</script>

<template>
    <Card radius="24px" pad="28px 30px" class="flex flex-col">
        <div class="mb-1.5 flex items-start justify-between">
            <div>
                <div class="text-[16px] font-semibold tracking-[-0.01em]">Sprint pacing</div>
                <div class="mt-[3px] text-[12.5px] text-faint-2">{{ subtitle }}</div>
            </div>
            <div class="flex flex-col items-end gap-1.5 font-mono text-[11px] font-medium">
                <span class="inline-flex items-center gap-1.5 text-muted">
                    <span class="inline-block h-0.5 w-3.5 rounded-sm bg-ink-soft"></span>committed estimate
                </span>
                <span class="inline-flex items-center gap-1.5 text-muted">
                    <span class="inline-block h-2 w-3.5 rounded-sm bg-accent-tint-2"></span>accumulated actual
                </span>
                <span class="inline-flex items-center gap-1.5 text-faint">
                    <span class="inline-block w-3.5 border-t-[1.5px] border-dashed border-behind"></span>ideal pace
                </span>
            </div>
        </div>
        <div class="mt-2 flex flex-1 items-center">
            <svg viewBox="0 0 360 200" width="100%" class="block">
                <g stroke="#f0ece4" stroke-width="1">
                    <line x1="24" y1="168" x2="344" y2="168" />
                    <line x1="24" y1="132" x2="344" y2="132" />
                    <line x1="24" y1="96" x2="344" y2="96" />
                    <line x1="24" y1="60" x2="344" y2="60" />
                    <line x1="24" y1="24" x2="344" y2="24" />
                </g>
                <g font-family="var(--font-mono)" font-size="8.5" fill="#c2bdb1" text-anchor="end">
                    <text x="19" y="171">0</text>
                    <text x="19" y="135">30</text>
                    <text x="19" y="99">60</text>
                    <text x="19" y="63">90</text>
                    <text x="19" y="27">120</text>
                </g>
                <line x1="24" y1="168" x2="344" y2="38.4" stroke="#b18749" stroke-width="1.5" stroke-dasharray="4 4" />
                <path d="M24,168 L56,156 L88,144 L120,136.8 L152,122.4 L184,105.6 L216,92.4 L248,74.4 L280,60 L280,168 Z" fill="#c9c2ee" opacity=".28" />
                <polyline
                    points="24,168 56,156 88,144 120,136.8 152,122.4 184,105.6 216,92.4 248,74.4 280,60"
                    fill="none"
                    stroke="#8074cf"
                    stroke-width="2.25"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                />
                <circle cx="280" cy="60" r="4" fill="#8074cf" stroke="#fff" stroke-width="2" />
                <polyline
                    points="24,48 56,48 88,48 120,48 152,43.2 184,43.2 216,43.2 248,38.4 280,38.4 312,38.4 344,38.4"
                    fill="none"
                    stroke="#3f3d38"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                />
                <line x1="280" y1="20" x2="280" y2="176" stroke="#d8d3c8" stroke-width="1" stroke-dasharray="2 3" />
                <g font-family="var(--font-mono)" font-size="8.5" fill="#a8a498">
                    <text x="24" y="190">d0</text>
                    <text x="120" y="190" text-anchor="middle">d4</text>
                    <text x="280" y="190" text-anchor="middle" fill="#6d6a63">today</text>
                    <text x="344" y="190" text-anchor="end">d10</text>
                </g>
            </svg>
        </div>
        <div class="mt-3.5 flex gap-7 border-t border-divider pt-4">
            <div>
                <div class="mb-[5px] font-mono text-[10.5px] text-faint-2">DELIVERED</div>
                <div class="font-mono text-[17px] font-semibold tabular-nums">
                    {{ delivered }}<span class="text-[13px] text-faint-4">{{ deliveredOf }}</span>
                </div>
            </div>
            <div>
                <div class="mb-[5px] font-mono text-[10.5px] text-faint-2">PACE</div>
                <div class="font-mono text-[17px] font-semibold tabular-nums text-track">{{ pace }}</div>
            </div>
            <div>
                <div class="mb-[5px] font-mono text-[10.5px] text-faint-2">PROJECTED</div>
                <div class="font-mono text-[17px] font-semibold tabular-nums">{{ projected }}</div>
            </div>
        </div>
    </Card>
</template>

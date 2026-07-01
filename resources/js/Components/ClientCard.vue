<script setup>
import Card from './Card.vue';
import ProgressBar from './ProgressBar.vue';

const props = defineProps({
    initials: { type: String, default: '' },
    name: { type: String, default: '' },
    meta: { type: String, default: '' },
    hours: { type: [String, Number], default: '' },
    target: { type: [String, Number], default: '' },
    pct: { type: Number, default: 0 },
    tone: { type: String, default: 'track' }, // track | behind
    status: { type: String, default: '' },
    daysLeft: { type: String, default: '' },
});

const avatar = {
    track: 'bg-track-tint text-track',
    behind: 'bg-behind-tint text-behind',
};
const statusColor = { track: 'text-track', behind: 'text-behind' };
</script>

<template>
    <Card radius="22px" pad="24px 26px">
        <div class="mb-[18px] flex items-start justify-between">
            <div class="flex items-center gap-[13px]">
                <div
                    class="flex h-10 w-10 items-center justify-center rounded-[13px] font-mono text-[15px] font-semibold"
                    :class="avatar[tone]"
                >
                    {{ initials }}
                </div>
                <div>
                    <div class="text-[16px] font-semibold tracking-[-0.01em]">{{ name }}</div>
                    <div class="mt-0.5 text-[12px] text-faint-2">{{ meta }}</div>
                </div>
            </div>
            <div class="text-right">
                <div class="font-mono text-[15px] font-semibold tabular-nums">
                    {{ hours }}<span class="text-[12px] text-faint-4"> / {{ target }}h</span>
                </div>
                <div class="mt-[3px] text-[11px] text-faint-2">this month</div>
            </div>
        </div>
        <ProgressBar :value="pct" :tone="tone" class="mb-[9px]" />
        <div class="flex justify-between font-mono text-[11.5px] font-medium text-faint">
            <span :class="statusColor[tone]">{{ status }}</span>
            <span>{{ daysLeft }}</span>
        </div>
    </Card>
</template>

<script setup>
import Card from './Card.vue';

defineProps({
    title: { type: String, default: 'Team · this sprint' },
    tag: { type: String, default: 'read-only' },
    // member: { initials, name, hours, pct, tone, avatarTone, aging, agingTone }
    members: { type: Array, default: () => [] },
});

const avatar = {
    accent: 'bg-accent-chip text-accent',
    track: 'bg-track-tint text-track',
    behind: 'bg-behind-tint text-behind',
    neutral: 'bg-[#eceae4] text-[#8a8578]',
};
const fill = { track: 'bg-track', behind: 'bg-behind', accent: 'bg-accent' };
const agingColor = { behind: 'text-behind', neutral: 'text-[#8a8578]' };
</script>

<template>
    <Card radius="24px" pad="24px 12px 16px">
        <div class="flex items-center justify-between px-4 pb-1">
            <div class="text-[16px] font-semibold tracking-[-0.01em]">{{ title }}</div>
            <div class="font-mono text-[11px] font-medium text-faint-3">{{ tag }}</div>
        </div>
        <div class="flex flex-col">
            <div
                v-for="m in members"
                :key="m.initials"
                class="flex items-center gap-3.5 border-t border-divider-soft px-4 py-[15px]"
            >
                <div
                    class="flex h-[34px] w-[34px] flex-none items-center justify-center rounded-[11px] font-mono text-[12px] font-semibold"
                    :class="avatar[m.avatarTone]"
                >
                    {{ m.initials }}
                </div>
                <div class="min-w-0 flex-1">
                    <div class="flex items-baseline justify-between">
                        <span class="text-[14px] font-semibold">{{ m.name }}</span>
                        <span class="font-mono text-[12px] font-medium tabular-nums text-muted">{{ m.hours }}</span>
                    </div>
                    <div class="mt-2 h-[5px] overflow-hidden rounded-full bg-sunken">
                        <div class="h-full rounded-full" :class="fill[m.tone]" :style="{ width: m.pct + '%' }"></div>
                    </div>
                </div>
                <div class="w-[78px] flex-none text-right">
                    <div class="font-mono text-[12px] font-medium tabular-nums" :class="agingColor[m.agingTone]">
                        {{ m.aging }}
                    </div>
                    <div class="mt-[3px] text-[10.5px] text-faint-3">aging</div>
                </div>
            </div>
        </div>
    </Card>
</template>

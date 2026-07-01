<script setup>
import Card from './Card.vue';
import Chip from './Chip.vue';

defineProps({
    title: { type: String, default: 'Work items' },
    countLabel: { type: String, default: 'In progress · 5' },
    // item: { key, title, type: 'Feature'|'Bug'|'Chore', billable, est, act,
    //         actTone: 'behind'|'track'|'muted', priority, highlight }
    items: { type: Array, default: () => [] },
});

const cols = 'grid-cols-[64px_1fr_96px_108px_30px]';
const typeDot = { Feature: 'bg-accent-soft', Bug: 'bg-behind', Chore: 'bg-faint-2' };
const actColor = { behind: 'text-behind', track: 'text-track', muted: 'text-muted' };
</script>

<template>
    <Card radius="24px" pad="10px 10px 12px">
        <div class="flex items-center justify-between px-5 pb-3.5 pt-4">
            <div class="text-[16px] font-semibold tracking-[-0.01em]">{{ title }}</div>
            <Chip tone="accent">{{ countLabel }}</Chip>
        </div>

        <div
            class="grid gap-3 px-5 py-2 font-mono text-[10px] font-semibold uppercase tracking-[0.1em] text-faint-3"
            :class="cols"
        >
            <span>Key</span>
            <span>Title</span>
            <span class="text-right">Est → act</span>
            <span class="text-right">Priority</span>
            <span></span>
        </div>

        <div class="flex flex-col">
            <div
                v-for="item in items"
                :key="item.key"
                class="grid items-center gap-3 rounded-[14px] border-t border-divider-soft px-5 py-3.5"
                :class="[cols, item.highlight ? 'bg-surface-soft' : '']"
            >
                <span class="font-mono text-[12px] font-semibold text-muted">{{ item.key }}</span>
                <div class="min-w-0">
                    <div class="flex items-center gap-2">
                        <span class="h-[7px] w-[7px] flex-none rounded-sm" :class="typeDot[item.type]"></span>
                        <span class="truncate text-[14px] font-medium">{{ item.title }}</span>
                    </div>
                    <div class="mt-[3px] font-mono text-[11px] text-faint-3">
                        {{ item.type }} · {{ item.billable ? 'billable' : 'non-billable' }}
                    </div>
                </div>
                <div class="text-right font-mono text-[13px] font-medium tabular-nums text-muted">
                    {{ item.est.toFixed(1) }} → <span :class="actColor[item.actTone]">{{ item.act.toFixed(1) }}</span>
                </div>
                <div class="text-right">
                    <span class="rounded-[7px] bg-divider px-[9px] py-[5px] font-mono text-[11px] font-medium text-[#8a8578]">{{
                        item.priority
                    }}</span>
                </div>
                <div class="flex justify-center">
                    <span
                        v-if="item.billable"
                        class="h-2 w-2 rounded-full bg-accent"
                        title="billable"
                    ></span>
                    <span
                        v-else
                        class="h-2 w-2 rounded-full border-[1.5px] border-[#cbc6ba]"
                        title="non-billable"
                    ></span>
                </div>
            </div>
        </div>
    </Card>
</template>

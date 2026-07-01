<script setup>
import Card from './Card.vue';

defineProps({
    active: {
        type: Object,
        default: () => ({ key: 'FYL-231', title: 'Refactor invoice PDF export', time: '00:00:00' }),
    },
    paused: {
        type: Array,
        default: () => [],
    },
    hint: { type: String, default: '' },
});
defineEmits(['stop']);
</script>

<template>
    <Card radius="24px" pad="26px 26px 30px">
        <div class="mb-1.5 flex items-center justify-between">
            <div class="font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">Timer stack</div>
            <div class="text-[12px] text-faint-2">{{ paused.length + 1 }} running</div>
        </div>
        <p class="mb-5 text-[12.5px] leading-[1.5] text-faint-2">
            Start a timer while one runs and it pushes on top. Stop it and the one beneath resumes.
        </p>

        <!-- active (top of stack) -->
        <div class="relative z-30 rounded-[18px] border-[1.5px] border-[#d9d3f4] bg-accent-wash px-5 py-[18px]">
            <div class="mb-3 flex items-center justify-between">
                <div class="inline-flex items-center gap-2">
                    <span class="h-2 w-2 rounded-full bg-accent" style="animation: fyl-pulse 2s ease-in-out infinite"></span>
                    <span class="font-mono text-[11px] font-semibold uppercase tracking-[0.1em] text-accent-deep">Active</span>
                </div>
                <span class="rounded-[7px] bg-accent-chip px-[9px] py-1 font-mono text-[12px] font-semibold text-accent">{{
                    active.key
                }}</span>
            </div>
            <div class="mb-3.5 text-[15px] font-semibold tracking-[-0.01em]">{{ active.title }}</div>
            <div class="flex items-end justify-between">
                <span class="font-mono text-[32px] font-semibold tabular-nums tracking-[-0.02em] text-accent">{{
                    active.time
                }}</span>
                <button
                    class="flex h-10 w-10 cursor-pointer items-center justify-center rounded-[13px] border-0 bg-accent shadow-btn"
                    @click="$emit('stop')"
                >
                    <span class="block h-[11px] w-[11px] rounded-sm bg-white"></span>
                </button>
            </div>
        </div>

        <!-- paused, nested beneath -->
        <div
            v-for="(row, i) in paused"
            :key="row.key"
            class="relative rounded-b-2xl border border-t-0 border-border-soft px-4 pb-3.5 pt-[15px]"
            :class="i === 0 ? 'z-20 bg-surface-soft' : 'z-10 bg-[#f7f6f2] opacity-90'"
            :style="{ marginLeft: (i + 1) * 8 + 'px', marginRight: (i + 1) * 8 + 'px' }"
        >
            <div class="flex items-center justify-between">
                <div class="flex min-w-0 items-center gap-2.5">
                    <span class="font-mono text-[11px] font-medium text-faint-3">paused</span>
                    <span class="font-mono text-[11px] font-semibold text-[#8a8578]">{{ row.key }}</span>
                    <span class="truncate text-[13px] font-medium text-muted">{{ row.title }}</span>
                </div>
                <span class="flex-none font-mono text-[14px] font-medium tabular-nums text-[#8a8578]">{{ row.time }}</span>
            </div>
        </div>

        <div v-if="hint" class="mt-3.5 text-center font-mono text-[11px] font-medium text-faint-3">{{ hint }}</div>
    </Card>
</template>

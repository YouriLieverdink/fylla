<script setup>
import { computed } from 'vue';
import { Link, usePage } from '@inertiajs/vue3';

const url = computed(() => usePage().url);
const tabs = [
    { label: 'Personal', href: '/' },
    { label: 'Capacity', href: '/capacity' },
    { label: 'Projects', href: '/projects' },
];

// '/' matches only the root; other tabs match their prefix.
function active(href) {
    return href === '/' ? url.value === '/' : url.value.startsWith(href);
}
</script>

<template>
    <div class="flex items-center gap-3.5">
        <div class="relative h-[34px] w-[34px] rounded-[11px] bg-accent shadow-[0_5px_15px_-5px_rgba(108,95,201,0.6)]">
            <div class="absolute inset-0 flex items-center justify-center">
                <div
                    class="h-3 w-3 rounded-full border-[2.5px] border-white border-t-transparent"
                    style="transform: rotate(35deg)"
                ></div>
            </div>
        </div>
        <span class="text-[21px] font-semibold tracking-[-0.02em]">Fylla</span>
        <nav class="ml-3 flex items-baseline gap-4">
            <Link
                v-for="tab in tabs"
                :key="tab.href"
                :href="tab.href"
                class="font-mono text-[11px] font-semibold uppercase tracking-[0.16em] transition"
                :class="active(tab.href) ? 'text-ink' : 'text-faint hover:text-muted'"
            >
                {{ tab.label }}
            </Link>
        </nav>
    </div>
</template>

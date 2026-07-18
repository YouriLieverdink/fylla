<script setup>
import { computed, ref, watch } from 'vue';
import { Link, usePage } from '@inertiajs/vue3';

const url = computed(() => usePage().url);
// Two lenses (CONTEXT.md): personal utilization vs. team-aggregate delivery.
// Only one lens's tabs show at a time; Settings is a gear in the header.
const groups = {
    Personal: [
        { label: 'Worklist', href: '/' },
        { label: 'Utilization', href: '/utilization' },
        { label: 'Capacity', href: '/capacity' },
    ],
    Team: [
        { label: 'Estimation', href: '/estimation' },
        { label: 'Clients', href: '/clients' },
        { label: 'Delivery', href: '/delivery' },
    ],
};

function lensOf(u) {
    return groups.Team.some((t) => u.startsWith(t.href)) ? 'Team' : 'Personal';
}

// Default to the current page's lens, and follow cross-lens navigation (e.g.
// browser back). Switching the toggle only swaps visible tabs — no navigation.
const lens = ref(lensOf(url.value));
watch(url, (u) => (lens.value = lensOf(u)));

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

        <button
            type="button"
            :aria-label="`Switch to ${lens === 'Personal' ? 'Team' : 'Personal'} lens`"
            class="ml-3 flex items-center gap-1.5 rounded-full bg-divider-soft/60 px-2.5 py-1 font-mono text-[10px] font-semibold uppercase tracking-[0.16em] text-muted transition hover:text-ink"
            @click="lens = lens === 'Personal' ? 'Team' : 'Personal'"
        >
            <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <path d="M17 3l4 4-4 4" />
                <path d="M21 7H7" />
                <path d="M7 21l-4-4 4-4" />
                <path d="M3 17h14" />
            </svg>
            {{ lens }}
        </button>

        <nav class="flex items-baseline gap-4">
            <Link
                v-for="tab in groups[lens]"
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

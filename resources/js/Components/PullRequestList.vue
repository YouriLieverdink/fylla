<script setup>
import { router } from '@inertiajs/vue3';
import { reactive, ref } from 'vue';
import Card from './Card.vue';
import Chip from './Chip.vue';

const props = defineProps({
    // [{ id, number, repo, title, url, head_ref, suggested_key, kendo_key, resolved_at }]
    pullRequests: { type: Array, default: () => [] },
    livePrIds: { type: Array, default: () => [] },
});

const opts = { preserveScroll: true };

// Per-PR manual-pick UI state, keyed by PR id.
const picking = reactive({}); // id → { q, results, loading }
const errors = reactive({}); // id → message

function confirmKey(pr) {
    resolve(pr, pr.suggested_key);
}

function resolve(pr, key) {
    router.post(`/pull-requests/${pr.id}/resolve`, { key }, {
        ...opts,
        onError: (e) => (errors[pr.id] = e.resolve ?? 'Could not resolve.'),
        onSuccess: () => {
            delete errors[pr.id];
            delete picking[pr.id];
        },
    });
}

function openPick(pr) {
    picking[pr.id] = { q: pr.suggested_key ?? '', results: [], loading: false };
    if (picking[pr.id].q) search(pr.id);
}

async function search(id) {
    const state = picking[id];
    const q = state.q.trim();
    if (!q) {
        state.results = [];
        return;
    }
    state.loading = true;
    const res = await fetch(`/kendo/issues/search?q=${encodeURIComponent(q)}`, {
        headers: { Accept: 'application/json' },
    });
    state.results = res.ok ? await res.json() : [];
    state.loading = false;
}

function startTimer(pr) {
    router.post(`/pull-requests/${pr.id}/timer`, {}, opts);
}

const cols = 'grid-cols-[1fr_190px_96px]';
</script>

<template>
    <Card v-if="pullRequests.length" radius="24px" pad="10px 10px 12px">
        <div class="flex items-center justify-between px-5 pb-3.5 pt-4">
            <div class="text-[16px] font-semibold tracking-[-0.01em]">Pull requests</div>
            <Chip tone="accent">Review · {{ pullRequests.length }}</Chip>
        </div>

        <div
            class="grid gap-3 px-5 py-2 font-mono text-[10px] font-semibold uppercase tracking-[0.1em] text-faint-3"
            :class="cols"
        >
            <span>Title</span>
            <span>Kendo issue</span>
            <span></span>
        </div>

        <div class="flex flex-col">
            <div
                v-for="pr in pullRequests"
                :key="pr.id"
                class="grid items-start gap-3 rounded-[14px] border-t border-divider-soft px-5 py-3.5 transition"
                :class="livePrIds.includes(pr.id) ? 'bg-surface-soft' : 'hover:bg-surface-soft'"
            >
                <!-- title + repo/branch -->
                <div class="min-w-0">
                    <a
                        :href="pr.url"
                        target="_blank"
                        class="truncate text-[14px] font-medium hover:text-accent"
                        >{{ pr.title }}</a
                    >
                    <div class="mt-[3px] font-mono text-[11px] text-faint-3">
                        {{ pr.repo }}#{{ pr.number }}<span v-if="pr.head_ref"> · {{ pr.head_ref }}</span>
                    </div>
                </div>

                <!-- resolution -->
                <div class="min-w-0">
                    <!-- resolved -->
                    <a
                        v-if="pr.resolved_at"
                        :href="pr.kendo_url"
                        target="_blank"
                        title="Open in Kendo"
                        class="inline-block rounded-[7px] bg-accent-chip px-[9px] py-1 font-mono text-[12px] font-semibold text-accent hover:bg-accent-tint"
                        >{{ pr.kendo_key }}</a
                    >

                    <!-- unresolved: confirm suggested + manual pick -->
                    <template v-else>
                        <div v-if="!picking[pr.id]" class="flex flex-col gap-1.5">
                            <button
                                v-if="pr.suggested_key"
                                class="w-fit cursor-pointer rounded-[8px] border border-[#e0dbd0] bg-white px-2.5 py-1.5 font-mono text-[12px] font-semibold text-ink-soft hover:border-accent-tint-2"
                                @click="confirmKey(pr)"
                            >
                                Confirm {{ pr.suggested_key }}
                            </button>
                            <button
                                class="w-fit cursor-pointer font-mono text-[11px] text-faint-2 hover:text-accent"
                                @click="openPick(pr)"
                            >
                                {{ pr.suggested_key ? 'pick another' : 'link an issue' }}
                            </button>
                        </div>

                        <!-- manual live search -->
                        <div v-else class="flex flex-col gap-1.5">
                            <input
                                v-model="picking[pr.id].q"
                                type="text"
                                placeholder="Search Kendo issues…"
                                class="rounded-[8px] border border-[#e0dbd0] bg-white px-2 py-1 font-mono text-[12px] outline-none focus:border-accent-tint-2"
                                @input="search(pr.id)"
                                @keydown.esc="delete picking[pr.id]"
                            />
                            <div v-if="picking[pr.id].loading" class="font-mono text-[11px] text-faint-3">searching…</div>
                            <div v-else class="flex flex-col gap-1">
                                <button
                                    v-for="c in picking[pr.id].results"
                                    :key="c.id"
                                    class="w-fit cursor-pointer text-left font-mono text-[11px] text-ink-soft hover:text-accent"
                                    @click="resolve(pr, c.key)"
                                >
                                    <span class="font-semibold">{{ c.key }}</span> {{ c.title }}
                                </button>
                                <span
                                    v-if="picking[pr.id].q && !picking[pr.id].results.length"
                                    class="font-mono text-[11px] text-faint-3"
                                    >no matches</span
                                >
                            </div>
                        </div>
                        <span v-if="errors[pr.id]" class="mt-1 block font-mono text-[11px] text-behind">{{ errors[pr.id] }}</span>
                    </template>
                </div>

                <!-- action -->
                <div class="flex justify-end">
                    <span
                        v-if="livePrIds.includes(pr.id)"
                        class="inline-flex items-center gap-1.5 rounded-[10px] bg-accent-tint px-3 py-2 font-mono text-[11px] font-semibold uppercase tracking-[0.06em] text-accent-deep"
                    >
                        <span class="h-1.5 w-1.5 rounded-full bg-accent" style="animation: fyl-pulse 2s ease-in-out infinite"></span>
                        live
                    </span>
                    <button
                        v-else
                        :disabled="!pr.resolved_at"
                        :title="pr.resolved_at ? 'Start timer' : 'Resolve the linked Kendo issue first'"
                        class="inline-flex cursor-pointer items-center gap-[7px] rounded-[10px] border border-[#e0dbd0] bg-white px-[13px] py-2 font-sans text-[12.5px] font-semibold text-ink-soft transition hover:border-accent-tint-2 hover:bg-[#faf9fd] disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:border-[#e0dbd0] disabled:hover:bg-white"
                        @click="startTimer(pr)"
                    >
                        <span class="h-1.5 w-1.5 rounded-full bg-accent"></span>
                        Start
                    </button>
                </div>
            </div>
        </div>
    </Card>
</template>

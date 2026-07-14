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

const errors = reactive({}); // pr id → message

// Manual-pick modal state (one modal, whichever PR is being linked).
const pickingPr = ref(null);
const pickQ = ref('');
const pickResults = ref([]);
const pickLoading = ref(false);

function confirmKey(pr) {
    resolve(pr, pr.suggested_key);
}

function resolve(pr, key) {
    router.post(`/pull-requests/${pr.id}/resolve`, { key }, {
        ...opts,
        onError: (e) => (errors[pr.id] = e.resolve ?? 'Could not resolve.'),
        onSuccess: () => {
            delete errors[pr.id];
            closePick();
        },
    });
}

function openPick(pr) {
    pickingPr.value = pr;
    pickQ.value = pr.suggested_key ?? '';
    pickResults.value = [];
    if (pickQ.value) search();
}

function closePick() {
    pickingPr.value = null;
}

async function search() {
    const q = pickQ.value.trim();
    if (!q) {
        pickResults.value = [];
        return;
    }
    pickLoading.value = true;
    const res = await fetch(`/kendo/issues/search?q=${encodeURIComponent(q)}`, {
        headers: { Accept: 'application/json' },
    });
    pickResults.value = res.ok ? await res.json() : [];
    pickLoading.value = false;
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

                    <!-- unresolved: confirm suggested + manual pick (two buttons) -->
                    <template v-else>
                        <div class="flex items-center gap-2">
                            <button
                                v-if="pr.suggested_key"
                                class="cursor-pointer rounded-[8px] border border-[#e0dbd0] bg-white px-2.5 py-1.5 font-mono text-[12px] font-semibold text-ink-soft hover:border-accent-tint-2"
                                @click="confirmKey(pr)"
                            >
                                Confirm {{ pr.suggested_key }}
                            </button>
                            <button
                                class="cursor-pointer rounded-[8px] border border-[#e0dbd0] bg-white px-2.5 py-1.5 font-mono text-[12px] font-medium text-faint-2 hover:border-accent-tint-2 hover:text-accent"
                                @click="openPick(pr)"
                            >
                                {{ pr.suggested_key ? 'Pick another' : 'Link an issue' }}
                            </button>
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

    <!-- manual-pick modal: live Kendo search -->
    <Teleport to="body">
        <div
            v-if="pickingPr"
            class="fixed inset-0 z-50 flex items-start justify-center bg-black/30 p-4 pt-[15vh]"
            @click.self="closePick"
            @keydown.esc="closePick"
        >
            <div class="w-full max-w-[520px] rounded-[18px] border border-[#ebe7de] bg-surface p-5 shadow-[0_20px_60px_-15px_rgba(42,41,38,0.4)]">
                <div class="mb-1 flex items-center justify-between">
                    <div class="text-[15px] font-semibold tracking-[-0.01em]">Link Kendo issue</div>
                    <button class="cursor-pointer px-1 text-[18px] leading-none text-faint-2 hover:text-ink-soft" title="Close" @click="closePick">×</button>
                </div>
                <div class="mb-3 font-mono text-[11px] text-faint-3">{{ pickingPr.repo }}#{{ pickingPr.number }} · {{ pickingPr.title }}</div>

                <input
                    v-model="pickQ"
                    type="text"
                    autofocus
                    placeholder="Search Kendo issues…"
                    class="w-full rounded-[10px] border border-[#e0dbd0] bg-white px-3 py-2.5 font-mono text-[13px] outline-none focus:border-accent-tint-2"
                    @input="search"
                    @keydown.esc="closePick"
                />

                <div class="mt-3 flex max-h-[320px] flex-col gap-0.5 overflow-auto">
                    <div v-if="pickLoading" class="px-1 py-2 font-mono text-[12px] text-faint-3">searching…</div>
                    <template v-else>
                        <button
                            v-for="c in pickResults"
                            :key="c.id"
                            class="cursor-pointer rounded-[8px] px-2 py-2 text-left font-mono text-[12px] text-ink-soft hover:bg-surface-soft hover:text-accent"
                            @click="resolve(pickingPr, c.key)"
                        >
                            <span class="font-semibold">{{ c.key }}</span> {{ c.title }}
                        </button>
                        <span v-if="pickQ && !pickResults.length" class="px-1 py-2 font-mono text-[12px] text-faint-3">no matches</span>
                    </template>
                </div>

                <span v-if="pickingPr && errors[pickingPr.id]" class="mt-2 block font-mono text-[11px] text-behind">{{ errors[pickingPr.id] }}</span>
            </div>
        </div>
    </Teleport>
</template>

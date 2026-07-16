<script setup>
import { router, usePoll } from '@inertiajs/vue3';
import { reactive, ref } from 'vue';
import Card from '../Components/Card.vue';
import AppHeader from '../Components/AppHeader.vue';
import Chip from '../Components/Chip.vue';
import EmptyState from '../Components/EmptyState.vue';
import AppButton from '../Components/AppButton.vue';
import BillableMetric from '../Components/BillableMetric.vue';
import UtilizationTrendChart from '../Components/UtilizationTrendChart.vue';
import TimerStack from '../Components/TimerStack.vue';

const props = defineProps({
    // One ranked list of { kind:'issue'|'pr', id, title, reason, score, ... }.
    items: { type: Array, default: () => [] },
    timer: { type: Object, default: null },
    liveIssueIds: { type: Array, default: () => [] },
    livePrIds: { type: Array, default: () => [] },
    utilization: { type: Object, default: () => ({}) },
});

const opts = { preserveScroll: true };

// keep the list fresh when the 15-min scheduled sync fires; narrow only: leaves
// the running timer clock untouched (ticks locally off started_at)
usePoll(60000, { only: ['items', 'livePrIds', 'liveIssueIds', 'utilization'] });

function syncNow() {
    router.post('/sync', {}, opts);
}

// ids collide across morph types, so live checks are kind-scoped.
function isLive(item) {
    if (item.kind === 'draft') return false; // drafts are un-timeable (ADR-0012)
    return item.kind === 'issue'
        ? props.liveIssueIds.includes(item.id)
        : props.livePrIds.includes(item.id);
}

function startTimer(item) {
    const url = item.kind === 'issue' ? '/timers' : `/pull-requests/${item.id}/timer`;
    const body = item.kind === 'issue' ? { issue_id: item.id } : {};
    router.post(url, body, opts);
}

// minutes → "6h" / "1.5h"; em-dash when unset
function hrs(min) {
    if (min == null) return '—';
    const h = min / 60;
    return (Number.isInteger(h) ? h : h.toFixed(1)) + 'h';
}

// Kendo represents a fully-spent budget as null remaining (not 0) while the
// estimate stays set — that's the "over budget" signal, shown as red "0h".
function remSpent(item) {
    return item.remaining_minutes === 0
        || (item.estimated_minutes != null && item.remaining_minutes == null);
}
function remClass(item) {
    if (remSpent(item)) return 'text-over';
    if (item.remaining_minutes != null && item.estimated_minutes != null && item.remaining_minutes >= item.estimated_minutes) {
        return 'text-behind';
    }
    return 'text-track';
}

// type → the coloured square from the kit's work-item rows
const typeDot = { Feature: 'bg-accent-soft', Bug: 'bg-behind', Task: 'bg-faint-2' };

// --- issue editing: priority (Kendo write-through) + scheduling (local) ---
const PRIORITIES = ['Highest', 'High', 'Medium', 'Low', 'Lowest'];
const editing = ref(null); // issue id with the edit popover open
const draft = reactive({ title: '', priority: 'Medium', due_date: '', not_before: '', estimate_hours: '' });

// drafts are Fylla-owned (ADR-0012), issues write priority through to Kendo (ADR-0014)
function editUrl(item) {
    return item.kind === 'draft' ? `/drafts/${item.id}` : `/issues/${item.id}`;
}

// instant local-only write; never auto-cleared (ADR-0004)
function togglePin(item) {
    router.patch(editUrl(item), { up_next: !item.up_next }, {
        ...opts,
        onError: (e) => (errors[rowKey(item)] = e.priority ?? 'Could not update.'),
    });
}

// composite key: issue and draft ids can collide but share this popover
const rowKey = (item) => item.kind + '-' + item.id;

// flip the popover above the trigger when there isn't room below it
const dropUp = ref(false);

function openEdit(item, event) {
    editing.value = editing.value === rowKey(item) ? null : rowKey(item);
    const rect = event?.currentTarget.getBoundingClientRect();
    dropUp.value = rect ? window.innerHeight - rect.bottom < 360 : false;
    draft.title = item.title ?? '';
    draft.priority = item.priority ?? 'Medium';
    draft.due_date = item.due_date ?? '';
    draft.not_before = item.not_before ?? '';
    draft.estimate_hours = item.estimated_minutes != null ? item.estimated_minutes / 60 : '';
}

// priority/estimate may fail (Kendo write-through); dates/up_next always persist (ADR-0014).
// Drafts carry no estimate, so that field is omitted for them.
function saveEdit(item) {
    const payload = {
        priority: draft.priority,
        due_date: draft.due_date || null,
        not_before: draft.not_before || null,
    };
    if (item.kind === 'draft') {
        payload.title = draft.title.trim() || item.title; // title is a draft's whole content; never blank it
    } else {
        payload.estimated_minutes = draft.estimate_hours === '' ? null : Math.round(draft.estimate_hours * 60);
    }
    router.patch(editUrl(item), payload, {
        ...opts,
        onError: (e) => (errors[rowKey(item)] = e.priority ?? 'Could not save.'),
        onSuccess: () => {
            delete errors[rowKey(item)];
            editing.value = null;
        },
    });
}

// --- draft capture / removal (ADR-0012) ---
const newDraft = ref('');

function captureDraft() {
    const title = newDraft.value.trim();
    if (!title) return;
    router.post('/drafts', { title }, { ...opts, onSuccess: () => (newDraft.value = '') });
}

function deleteDraft(item) {
    router.delete(`/drafts/${item.id}`, opts);
}

// --- PR resolution (ported from PullRequestList) ---
const errors = reactive({}); // pr id → message
const pickingPr = ref(null);
const pickQ = ref('');
const pickResults = ref([]);
const pickLoading = ref(false);

function resolve(pr, key) {
    router.post(`/pull-requests/${pr.id}/resolve`, { key }, {
        ...opts,
        onError: (e) => (errors[rowKey(pr)] = e.resolve ?? 'Could not resolve.'),
        onSuccess: () => {
            delete errors[rowKey(pr)];
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
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-11">
        <AppHeader />

        <div class="mb-8">
            <h1 class="mb-3 text-[34px] font-bold leading-[1.05] tracking-[-0.03em]">Worklist</h1>
            <p class="max-w-[62ch] text-[15px] leading-[1.55] text-muted">
                Your billable utilization and every open work item — issues and pull requests — ranked
                top-to-bottom by urgency. Start a timer on any row to log time against it.
            </p>
        </div>

        <!-- metrics row -->
        <div class="mb-[22px] grid items-stretch gap-[22px] lg:grid-cols-[400px_1fr]">
            <BillableMetric
                :value="utilization.value"
                :status="utilization.status"
                :delta="utilization.delta"
                :delta-caption="utilization.deltaCaption"
                :target="utilization.target"
                :note="utilization.note"
                :week="utilization.week"
            />
            <UtilizationTrendChart :points="utilization.points" :target="utilization.target" />
        </div>

        <!-- timer stack -->
        <div class="mb-[22px]">
            <TimerStack
                :active="timer?.active ?? null"
                :paused="timer?.paused ?? []"
                @pause="router.post('/timers/pause', {}, opts)"
                @resume="router.post('/timers/resume', {}, opts)"
                @stop="router.post('/timers/stop', {}, opts)"
                @note="(text) => router.post('/timers/notes', { text }, opts)"
            />
        </div>

        <!-- draft capture: one gesture — type a to-do, hit enter (ADR-0012) -->
        <div class="mb-[22px] flex items-center gap-3 rounded-[16px] border border-divider-soft bg-surface px-4 py-3">
            <span class="flex-none font-mono text-[15px] leading-none text-faint-2">✎</span>
            <input
                v-model="newDraft"
                type="text"
                placeholder="Jot a to-do — a client to email, a person to talk to…"
                autocomplete="off"
                data-bwignore="true"
                data-1p-ignore
                data-lpignore="true"
                class="min-w-0 flex-1 bg-transparent text-[14px] outline-none placeholder:text-faint-3"
                @keydown.enter="captureDraft"
            />
            <button
                class="flex-none cursor-pointer rounded-[9px] bg-accent px-3.5 py-1.5 font-sans text-[12px] font-semibold text-white transition hover:bg-accent-deep"
                :class="{ invisible: !newDraft.trim() }"
                @click="captureDraft"
            >
                Add
            </button>
        </div>

        <!-- worklist -->
        <Card v-if="items.length" radius="24px" pad="10px 10px 12px">
            <div class="flex items-center justify-between px-5 pb-3.5 pt-4">
                <div class="text-[16px] font-semibold tracking-[-0.01em]">Worklist</div>
                <Chip tone="accent">{{ items.length }} items</Chip>
            </div>

            <div class="flex flex-col">
                <div
                    v-for="item in items"
                    :key="item.kind + item.id"
                    class="flex items-center gap-4 rounded-[14px] border-t border-divider-soft px-5 py-3.5 transition"
                    :class="isLive(item) ? 'bg-surface-soft' : 'hover:bg-surface-soft'"
                >
                    <!-- key / repo#number / draft marker -->
                    <span
                        class="w-[120px] flex-none truncate font-mono text-[12px] font-semibold text-muted"
                        :title="item.kind === 'issue' ? item.key : item.kind === 'pr' ? item.repo + '#' + item.number : 'Draft'"
                    >
                        <template v-if="item.kind === 'issue'">{{ item.key }}</template>
                        <template v-else-if="item.kind === 'pr'">{{ item.repo.split('/').pop() }}#{{ item.number }}</template>
                        <span v-else class="text-faint-3">draft</span>
                    </span>

                    <!-- title + reason -->
                    <div class="min-w-0 flex-1">
                        <div class="flex items-center gap-2">
                            <span
                                v-if="item.kind === 'issue'"
                                class="h-[7px] w-[7px] flex-none rounded-sm"
                                :class="typeDot[item.type] ?? 'bg-faint-2'"
                                :title="item.type"
                            ></span>
                            <span
                                v-else-if="item.kind === 'pr'"
                                class="flex-none rounded-[5px] bg-accent-chip px-[6px] py-[1px] font-mono text-[9px] font-semibold uppercase tracking-[0.08em] text-accent"
                                >PR</span
                            >
                            <span
                                v-else
                                class="flex-none rounded-[5px] bg-divider px-[6px] py-[1px] font-mono text-[9px] font-semibold uppercase tracking-[0.08em] text-faint-2"
                                >Draft</span
                            >
                            <a
                                v-if="item.kind !== 'draft'"
                                :href="item.kind === 'issue' ? item.kendo_url : item.url"
                                target="_blank"
                                class="truncate text-[14px] font-medium hover:text-accent"
                                >{{ item.title }}</a
                            >
                            <span v-else class="truncate text-[14px] font-medium">{{ item.title }}</span>
                        </div>
                        <div class="mt-[3px] font-mono text-[11px] text-faint-3">
                            {{ item.kind === 'issue' ? 'score ' + Math.round(item.score) : item.reason }}
                        </div>
                    </div>

                    <!-- issue/draft meta: (estimate) / priority / pin / edit -->
                    <template v-if="item.kind === 'issue' || item.kind === 'draft'">
                        <div v-if="item.kind === 'issue'" class="hidden flex-none text-right font-mono text-[12px] tabular-nums text-muted sm:block">
                            est {{ hrs(item.estimated_minutes) }} · rem
                            <span :class="remClass(item)">{{ remSpent(item) ? '0h' : hrs(item.remaining_minutes) }}</span>
                        </div>
                        <span
                            v-if="item.priority"
                            class="flex-none rounded-[7px] bg-divider px-[9px] py-[5px] font-mono text-[11px] font-medium text-[#8a8578]"
                            >{{ item.priority }}</span
                        >

                        <!-- pin: instant up_next toggle -->
                        <button
                            class="flex-none cursor-pointer rounded-[8px] px-1.5 py-1 text-[13px] leading-none transition hover:bg-divider"
                            :class="item.up_next ? 'opacity-100' : 'opacity-30 grayscale hover:opacity-70'"
                            :title="item.up_next ? 'Unpin from up next' : 'Pin to up next'"
                            @click="togglePin(item)"
                        >
                            📌
                        </button>

                        <!-- edit popover: priority + scheduling -->
                        <div class="relative flex-none">
                            <button
                                class="cursor-pointer rounded-[8px] px-1.5 py-1 font-mono text-[15px] leading-none text-faint-2 transition hover:bg-divider hover:text-ink-soft"
                                title="Edit priority & scheduling"
                                @click="openEdit(item, $event)"
                            >
                                ⋯
                            </button>

                            <!-- click-outside backdrop; popover sits above it (z-40 > z-30) -->
                            <div v-if="editing === rowKey(item)" class="fixed inset-0 z-30" @click="editing = null"></div>
                            <div
                                v-if="editing === rowKey(item)"
                                class="absolute right-0 z-40 w-[240px] rounded-[14px] border border-[#ebe7de] bg-surface p-3.5 shadow-[0_16px_44px_-14px_rgba(42,41,38,0.38)]"
                                :class="dropUp ? 'bottom-full mb-1.5' : 'top-full mt-1.5'"
                                @keydown.esc.window="editing = null"
                            >
                                <template v-if="item.kind === 'draft'">
                                    <label class="mb-1 block font-mono text-[10px] uppercase tracking-[0.08em] text-faint-3">Title</label>
                                    <textarea
                                        v-model="draft.title"
                                        rows="3"
                                        autocomplete="off"
                                        data-bwignore="true"
                                        data-1p-ignore
                                        data-lpignore="true"
                                        class="mb-3 w-full resize-none rounded-[9px] border border-[#e0dbd0] bg-white px-2.5 py-2 text-[12px] leading-snug outline-none focus:border-accent-tint-2"
                                    ></textarea>
                                </template>

                                <label class="mb-1 block font-mono text-[10px] uppercase tracking-[0.08em] text-faint-3">Priority</label>
                                <select
                                    v-model="draft.priority"
                                    class="mb-3 w-full rounded-[9px] border border-[#e0dbd0] bg-white px-2.5 py-2 font-mono text-[12px] outline-none focus:border-accent-tint-2"
                                >
                                    <option v-for="p in PRIORITIES" :key="p" :value="p">{{ p }}</option>
                                </select>

                                <label class="mb-1 block font-mono text-[10px] uppercase tracking-[0.08em] text-faint-3">Due date</label>
                                <div class="mb-3 flex items-center gap-1.5">
                                    <input
                                        v-model="draft.due_date"
                                        type="date"
                                        class="min-w-0 flex-1 rounded-[9px] border border-[#e0dbd0] bg-white px-2.5 py-2 font-mono text-[12px] outline-none focus:border-accent-tint-2"
                                    />
                                    <button v-if="draft.due_date" class="cursor-pointer px-1 text-[16px] leading-none text-faint-2 hover:text-behind" title="Clear" @click="draft.due_date = ''">×</button>
                                </div>

                                <label class="mb-1 block font-mono text-[10px] uppercase tracking-[0.08em] text-faint-3">Not before</label>
                                <div class="mb-3.5 flex items-center gap-1.5">
                                    <input
                                        v-model="draft.not_before"
                                        type="date"
                                        class="min-w-0 flex-1 rounded-[9px] border border-[#e0dbd0] bg-white px-2.5 py-2 font-mono text-[12px] outline-none focus:border-accent-tint-2"
                                    />
                                    <button v-if="draft.not_before" class="cursor-pointer px-1 text-[16px] leading-none text-faint-2 hover:text-behind" title="Clear" @click="draft.not_before = ''">×</button>
                                </div>

                                <!-- estimate is a Kendo-mirror field; drafts have none (ADR-0012) -->
                                <template v-if="item.kind === 'issue'">
                                    <label class="mb-1 block font-mono text-[10px] uppercase tracking-[0.08em] text-faint-3">Estimate (hours)</label>
                                    <div class="mb-3.5 flex items-center gap-1.5">
                                        <input
                                            v-model="draft.estimate_hours"
                                            type="number"
                                            min="0"
                                            step="0.25"
                                            placeholder="—"
                                            class="min-w-0 flex-1 rounded-[9px] border border-[#e0dbd0] bg-white px-2.5 py-2 font-mono text-[12px] outline-none focus:border-accent-tint-2"
                                        />
                                        <button v-if="draft.estimate_hours !== ''" class="cursor-pointer px-1 text-[16px] leading-none text-faint-2 hover:text-behind" title="Clear" @click="draft.estimate_hours = ''">×</button>
                                    </div>
                                </template>

                                <div class="flex items-center justify-between">
                                    <span v-if="errors[rowKey(item)]" class="font-mono text-[10px] text-behind">{{ errors[rowKey(item)] }}</span>
                                    <span v-else></span>
                                    <button
                                        class="cursor-pointer rounded-[9px] bg-accent px-3.5 py-1.5 font-sans text-[12px] font-semibold text-white transition hover:bg-accent-deep"
                                        @click="saveEdit(item)"
                                    >
                                        Done
                                    </button>
                                </div>
                            </div>
                        </div>
                    </template>

                    <!-- pr meta: resolution state -->
                    <template v-else-if="item.kind === 'pr'">
                        <div class="min-w-0 flex-none">
                            <a
                                v-if="item.resolved_at"
                                :href="item.kendo_url"
                                target="_blank"
                                title="Open in Kendo"
                                class="inline-block rounded-[7px] bg-accent-chip px-[9px] py-1 font-mono text-[12px] font-semibold text-accent hover:bg-accent-tint"
                                >{{ item.kendo_key }}</a
                            >
                            <template v-else>
                                <div class="flex items-center gap-2">
                                    <button
                                        v-if="item.suggested_key"
                                        class="cursor-pointer rounded-[8px] border border-[#e0dbd0] bg-white px-2.5 py-1.5 font-mono text-[12px] font-semibold text-ink-soft hover:border-accent-tint-2"
                                        @click="resolve(item, item.suggested_key)"
                                    >
                                        Confirm {{ item.suggested_key }}
                                    </button>
                                    <button
                                        class="cursor-pointer rounded-[8px] border border-[#e0dbd0] bg-white px-2.5 py-1.5 font-mono text-[12px] font-medium text-faint-2 hover:border-accent-tint-2 hover:text-accent"
                                        @click="openPick(item)"
                                    >
                                        {{ item.suggested_key ? 'Pick another' : 'Link an issue' }}
                                    </button>
                                </div>
                                <span v-if="errors[rowKey(item)]" class="mt-1 block font-mono text-[11px] text-behind">{{ errors[rowKey(item)] }}</span>
                            </template>
                        </div>
                    </template>

                    <!-- action -->
                    <div class="flex w-[92px] flex-none justify-end">
                        <!-- drafts are un-timeable (ADR-0012): remove-when-done, no timer -->
                        <button
                            v-if="item.kind === 'draft'"
                            class="inline-flex cursor-pointer items-center gap-[7px] rounded-[10px] border border-[#e0dbd0] bg-white px-[13px] py-2 font-sans text-[12.5px] font-semibold text-ink-soft transition hover:border-behind hover:text-behind"
                            title="Mark done & remove"
                            @click="deleteDraft(item)"
                        >
                            <span class="text-[13px] leading-none text-accent">✓</span>
                            Done
                        </button>
                        <span
                            v-else-if="isLive(item)"
                            class="inline-flex items-center gap-1.5 rounded-[10px] bg-accent-tint px-3 py-2 font-mono text-[11px] font-semibold uppercase tracking-[0.06em] text-accent-deep"
                        >
                            <span class="h-1.5 w-1.5 rounded-full bg-accent" style="animation: fyl-pulse 2s ease-in-out infinite"></span>
                            live
                        </span>
                        <button
                            v-else
                            :disabled="item.kind === 'pr' && !item.resolved_at"
                            :title="item.kind === 'pr' && !item.resolved_at ? 'Resolve the linked Kendo issue first' : 'Start timer'"
                            class="inline-flex cursor-pointer items-center gap-[7px] rounded-[10px] border border-[#e0dbd0] bg-white px-[13px] py-2 font-sans text-[12.5px] font-semibold text-ink-soft transition hover:border-accent-tint-2 hover:bg-[#faf9fd] disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:border-[#e0dbd0] disabled:hover:bg-white"
                            @click="startTimer(item)"
                        >
                            <span class="h-1.5 w-1.5 rounded-full bg-accent"></span>
                            Start
                        </button>
                    </div>
                </div>
            </div>
        </Card>

        <EmptyState
            v-else
            title="Nothing on the worklist"
            text="No open issues or pull requests right now. Pull the latest from your issue tracker to populate this view."
        >
            <template #action>
                <AppButton variant="primary" size="sm" @click="syncNow">Sync now</AppButton>
            </template>
        </EmptyState>
    </div>

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

                <span v-if="pickingPr && errors[rowKey(pickingPr)]" class="mt-2 block font-mono text-[11px] text-behind">{{ errors[rowKey(pickingPr)] }}</span>
            </div>
        </div>
    </Teleport>
</template>

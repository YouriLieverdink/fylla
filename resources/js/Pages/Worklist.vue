<script setup>
import { router, usePoll } from '@inertiajs/vue3';
import { computed, reactive, ref, watch } from 'vue';
import Card from '../Components/Card.vue';
import AppHeader from '../Components/AppHeader.vue';
import Chip from '../Components/Chip.vue';
import EmptyState from '../Components/EmptyState.vue';
import AppButton from '../Components/AppButton.vue';
import BillableMetric from '../Components/BillableMetric.vue';
import UtilizationTrendChart from '../Components/UtilizationTrendChart.vue';
import TimerStack from '../Components/TimerStack.vue';
import { usePageCursor } from '../Composables/usePageCursor';
import { useModalGuard } from '../Composables/useModalGuard';
import { useAction } from '../Composables/useAction';

const props = defineProps({
    // One ranked list of { kind:'issue'|'pr', id, title, reason, score, ... }.
    items: { type: Array, default: () => [] },
    timer: { type: Object, default: null },
    liveIssueIds: { type: Array, default: () => [] },
    livePrIds: { type: Array, default: () => [] },
    utilization: { type: Object, default: () => ({}) },
    // { kendo_id, name } — promote targets for a draft (ADR-0012).
    projects: { type: Array, default: () => [] },
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

// Start eligibility, shared by the row's Start button (:disabled) and the `t`
// keybinding (#44): drafts are un-timeable, a live row is already running, an
// unresolved PR must resolve first.
function canStart(item) {
    return item.kind !== 'draft' && !isLive(item) && !(item.kind === 'pr' && !item.resolved_at);
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

// composite key: issue and draft ids can collide but share this modal
const rowKey = (item) => item.kind + '-' + item.id;

// j/k/digit cursor over the page's focus sequence: the two summary cards first,
// then the worklist rows ([utilization, timer, ...items]). First j lands on the
// utilization card; digits count from there (1 = utilization, 2 = timer, 3 = row 1).
// Cards carry a `focusKey`; rows fall back to rowKey so id-tracking still holds.
const cards = {
    utilization: { focusKey: 'utilization' },
    timer: { focusKey: 'timer' },
};
const focusTargets = computed(() => [cards.utilization, cards.timer, ...props.items]);
const cursor = usePageCursor(() => focusTargets.value, (t) => t.focusKey ?? rowKey(t));

// lock body scroll while the edit modal is open
watch(editing, (open) => {
    document.body.style.overflow = open ? 'hidden' : '';
});

function openEdit(item) {
    editing.value = editing.value === rowKey(item) ? null : rowKey(item);
    draft.title = item.title ?? '';
    draft.priority = item.priority ?? 'Medium';
    draft.due_date = item.due_date ?? '';
    draft.not_before = item.not_before ?? '';
    draft.estimate_hours = item.estimated_minutes != null ? item.estimated_minutes / 60 : '';
    if (item.kind === 'draft') {
        promoteProject.value = '';
        promoteQuery.value = '';
    }
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

// promote a draft into a Kendo issue (ADR-0012); one-way, draft is removed on success
// search-select the promote target so it scales past a dropdown's worth of projects
const promoteProject = ref(''); // selected kendo_id, '' = none
const promoteQuery = ref('');
const promoteMatches = computed(() => {
    const q = promoteQuery.value.trim().toLowerCase();
    const list = q ? props.projects.filter((p) => p.name.toLowerCase().includes(q)) : props.projects;
    return list.slice(0, 8); // a short menu; refine the query for the rest
});
function pickProject(p) {
    promoteProject.value = p.kendo_id;
    promoteQuery.value = p.name;
}
// resolve the target from an explicit pick, else an exact or single query match —
// so typing a name and hitting Promote works without a separate dropdown click
function resolvePromoteTarget() {
    if (promoteProject.value) return promoteProject.value;
    const q = promoteQuery.value.trim().toLowerCase();
    if (!q) return null;
    const exact = props.projects.find((p) => p.name.toLowerCase() === q);
    if (exact) return exact.kendo_id;
    return promoteMatches.value.length === 1 ? promoteMatches.value[0].kendo_id : null;
}
function promote(item) {
    const pid = resolvePromoteTarget();
    if (!pid) {
        errors[rowKey(item)] = promoteQuery.value.trim()
            ? 'Multiple projects match — pick one from the list.'
            : 'Search and pick a project first.';
        return;
    }
    router.post(`/drafts/${item.id}/promote`, { project_id: pid }, {
        ...opts,
        onError: (e) => (errors[rowKey(item)] = e.promote ?? e.project_id ?? 'Could not promote.'),
        onSuccess: () => {
            delete errors[rowKey(item)];
            editing.value = null;
        },
    });
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

// --- ad-hoc timing (ADR-0015): search any Kendo issue, pick it, timer starts ---
// separate state from the PR pick modal (pickingPr) so they never collide
const adhocOpen = ref(false);
const adhocQ = ref('');
const adhocResults = ref([]);
const adhocLoading = ref(false);

// Modal guard (#43): the three blocking modals on this page (edit — which holds
// the promote-pick — plus manual-pick and ad-hoc) suppress every keybinding
// beneath the scrim while open; Escape (each modal's own handler) is the exit.
useModalGuard(() => editing.value !== null);
useModalGuard(() => pickingPr.value !== null);
useModalGuard(() => adhocOpen.value);

function openAdhoc() {
    adhocOpen.value = true;
    adhocQ.value = '';
    adhocResults.value = [];
}

async function adhocSearch() {
    const q = adhocQ.value.trim();
    if (!q) {
        adhocResults.value = [];
        return;
    }
    adhocLoading.value = true;
    const res = await fetch(`/kendo/issues/search?q=${encodeURIComponent(q)}`, {
        headers: { Accept: 'application/json' },
    });
    adhocResults.value = res.ok ? await res.json() : [];
    adhocLoading.value = false;
}

function timeAdhoc(c) {
    router.post('/timers/adhoc', { key: c.key }, {
        ...opts,
        onSuccess: () => (adhocOpen.value = false),
    });
}

// --- Worklist keyset (#44, table #35) ---
// The one page that earns a full keyset. Per-item verbs act over the cursor;
// page verbs drive the timer / capture surfaces. All register in the `worklist`
// scope and ride the layout's guarded listener (suppressed while typing / under
// a modal). The focus/timer inputs live in native fields or TimerStack.
const captureInput = ref(null);
const timerStack = ref(null);

function toggleTimer() {
    if (!props.timer?.active) return;
    router.post(props.timer.active.running ? '/timers/pause' : '/timers/resume', {}, opts);
}
function stopTimer() {
    if (props.timer?.active) router.post('/timers/stop', {}, opts);
}

// The focused work item, or null when the cursor is unset or parked on a summary
// card — so a per-item verb off a real row is a no-op (#44).
function currentRow() {
    const t = cursor.current.value;
    return t && t.kind ? t : null;
}

// per-item verbs (over cursor.current)
useAction({ id: 'wl:timer', label: 'Start timer', keys: 't', scope: 'worklist', run: () => {
    const it = currentRow();
    if (it && canStart(it)) startTimer(it);
} });
useAction({ id: 'wl:open', label: 'Open work item', keys: 'o', scope: 'worklist', run: () => {
    const it = currentRow();
    if (!it || it.kind === 'draft') return; // drafts have no external link
    window.open(it.kind === 'issue' ? it.kendo_url : it.url, '_blank');
} });
useAction({ id: 'wl:edit', label: 'Edit priority & scheduling', keys: 'e', scope: 'worklist', run: () => {
    const it = currentRow();
    if (it && it.kind !== 'pr') openEdit(it); // PRs have no edit popover
} });
useAction({ id: 'wl:up-next', label: 'Toggle up next', keys: 'u', scope: 'worklist', run: () => {
    const it = currentRow();
    if (it && it.kind !== 'pr') togglePin(it);
} });
useAction({ id: 'wl:promote', label: 'Promote draft', keys: 'm', scope: 'worklist', run: () => {
    const it = currentRow();
    if (it && it.kind === 'draft') openEdit(it); // Promote lives in the edit modal (ADR-0012)
} });
useAction({ id: 'wl:resolve', label: 'Resolve PR', keys: 'r', scope: 'worklist', run: () => {
    const it = currentRow();
    if (!it || it.kind !== 'pr' || it.resolved_at) return;
    it.suggested_key ? resolve(it, it.suggested_key) : openPick(it);
} });
useAction({ id: 'wl:done', label: 'Mark draft done', keys: 'd', scope: 'worklist', run: () => {
    const it = currentRow();
    // confirm-gated: no single-keystroke data loss (#33 refined by #35).
    // ponytail: native confirm; swap for an in-app dialog if the design demands.
    if (it && it.kind === 'draft' && window.confirm(`Mark "${it.title}" done?`)) deleteDraft(it);
} });

// page verbs
useAction({ id: 'wl:capture', label: 'Capture draft', keys: 'c', scope: 'worklist', run: () => captureInput.value?.focus() });
useAction({ id: 'wl:adhoc', label: 'Log time on another task', keys: 'a', scope: 'worklist', run: openAdhoc });
useAction({ id: 'wl:pause', label: 'Pause / resume timer', keys: 'p', scope: 'worklist', run: toggleTimer });
useAction({ id: 'wl:stop', label: 'Stop timer', keys: 's', scope: 'worklist', run: stopTimer });
useAction({ id: 'wl:note', label: 'Add timer note', keys: 'n', scope: 'worklist', run: () => timerStack.value?.focusNote?.() });
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
        <div
            data-row="utilization"
            class="mb-[22px] grid scroll-my-12 items-stretch gap-[22px] lg:grid-cols-[400px_1fr]"
        >
            <BillableMetric
                :value="utilization.value"
                :status="utilization.status"
                :delta="utilization.delta"
                :delta-caption="utilization.deltaCaption"
                :target="utilization.target"
                :note="utilization.note"
                :week="utilization.week"
                :class="cursor.isActive(cards.utilization) && 'ring-2 ring-accent'"
            />
            <UtilizationTrendChart
                :points="utilization.points"
                :target="utilization.target"
                :class="cursor.isActive(cards.utilization) && 'ring-2 ring-accent'"
            />
        </div>

        <!-- timer stack -->
        <div
            data-row="timer"
            class="mb-[22px] scroll-my-12 rounded-[24px]"
            :class="cursor.isActive(cards.timer) && 'ring-2 ring-accent'"
        >
            <TimerStack
                ref="timerStack"
                :active="timer?.active ?? null"
                :paused="timer?.paused ?? []"
                @pause="router.post('/timers/pause', {}, opts)"
                @resume="router.post('/timers/resume', {}, opts)"
                @stop="router.post('/timers/stop', {}, opts)"
                @note="(text) => router.post('/timers/notes', { text }, opts)"
            />
            <!-- ad-hoc timing (ADR-0015): time a Kendo issue that isn't on the list -->
            <div class="mt-2 flex justify-end">
                <button
                    class="cursor-pointer font-mono text-[12px] text-faint-2 transition hover:text-accent"
                    @click="openAdhoc"
                >
                    + Log time on another task
                </button>
            </div>
        </div>

        <!-- draft capture: one gesture — type a to-do, hit enter (ADR-0012) -->
        <div class="mb-[22px] flex items-center gap-3 rounded-[16px] border border-divider-soft bg-surface px-4 py-3">
            <span class="flex-none font-mono text-[15px] leading-none text-faint-2">✎</span>
            <input
                ref="captureInput"
                v-model="newDraft"
                type="text"
                placeholder="Jot a to-do — a client to email, a person to talk to…"
                autocomplete="off"
                data-bwignore="true"
                data-1p-ignore
                data-lpignore="true"
                class="min-w-0 flex-1 bg-transparent text-[14px] outline-none placeholder:text-faint-3"
                @keydown.enter="captureDraft"
                @keydown.esc="captureInput?.blur()"
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
                    :data-row="rowKey(item)"
                    class="flex scroll-my-12 items-center gap-4 rounded-[14px] border-t border-divider-soft px-5 py-3.5 transition"
                    :class="[isLive(item) ? 'bg-surface-soft' : 'hover:bg-surface-soft', cursor.isActive(item) && 'ring-2 ring-inset ring-accent']"
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
                                @click="openEdit(item)"
                            >
                                ⋯
                            </button>

                            <!-- centered modal (matches the Clients add-project dialog) -->
                            <div
                                v-if="editing === rowKey(item)"
                                class="fixed inset-0 z-50 flex items-start justify-center bg-black/30 px-4 pt-[15vh]"
                                @click.self="editing = null"
                                @keydown.esc.window="editing = null"
                            >
                                <Card radius="18px" pad="18px 20px" class="max-h-[70vh] w-full max-w-[440px] overflow-y-auto">
                                    <div class="mb-3 flex items-center justify-between">
                                        <h2 class="text-[15px] font-semibold">{{ item.kind === 'draft' ? 'Edit draft' : 'Edit ' + item.key }}</h2>
                                        <button class="cursor-pointer font-mono text-[11px] uppercase tracking-[0.1em] text-faint-3 hover:text-ink" @click="editing = null">Close</button>
                                    </div>
                                <template v-if="item.kind === 'draft'">
                                    <label class="mb-1.5 block font-mono text-[11px] uppercase tracking-[0.08em] text-faint-3">Title</label>
                                    <textarea
                                        v-model="draft.title"
                                        rows="3"
                                        autocomplete="off"
                                        data-bwignore="true"
                                        data-1p-ignore
                                        data-lpignore="true"
                                        class="mb-4 w-full resize-none rounded-[11px] border border-[#e0dbd0] bg-surface px-3.5 py-2.5 text-[14px] leading-snug outline-none focus:border-accent"
                                    ></textarea>
                                </template>

                                <label class="mb-1.5 block font-mono text-[11px] uppercase tracking-[0.08em] text-faint-3">Priority</label>
                                <select
                                    v-model="draft.priority"
                                    class="mb-4 w-full rounded-[11px] border border-[#e0dbd0] bg-surface px-3.5 py-2.5 text-[14px] outline-none focus:border-accent"
                                >
                                    <option v-for="p in PRIORITIES" :key="p" :value="p">{{ p }}</option>
                                </select>

                                <label class="mb-1.5 block font-mono text-[11px] uppercase tracking-[0.08em] text-faint-3">Due date</label>
                                <div class="mb-4 flex items-center gap-2">
                                    <input
                                        v-model="draft.due_date"
                                        type="date"
                                        class="min-w-0 flex-1 rounded-[11px] border border-[#e0dbd0] bg-surface px-3.5 py-2.5 text-[14px] outline-none focus:border-accent"
                                    />
                                    <button v-if="draft.due_date" class="cursor-pointer px-1 text-[18px] leading-none text-faint-2 hover:text-behind" title="Clear" @click="draft.due_date = ''">×</button>
                                </div>

                                <label class="mb-1.5 block font-mono text-[11px] uppercase tracking-[0.08em] text-faint-3">Not before</label>
                                <div class="mb-4 flex items-center gap-2">
                                    <input
                                        v-model="draft.not_before"
                                        type="date"
                                        class="min-w-0 flex-1 rounded-[11px] border border-[#e0dbd0] bg-surface px-3.5 py-2.5 text-[14px] outline-none focus:border-accent"
                                    />
                                    <button v-if="draft.not_before" class="cursor-pointer px-1 text-[18px] leading-none text-faint-2 hover:text-behind" title="Clear" @click="draft.not_before = ''">×</button>
                                </div>

                                <!-- promote target: a draft has no project, so search + pick one (ADR-0012) -->
                                <template v-if="item.kind === 'draft' && projects.length">
                                    <label class="mb-1.5 block font-mono text-[11px] uppercase tracking-[0.08em] text-faint-3">Promote to project</label>
                                    <input
                                        v-model="promoteQuery"
                                        type="text"
                                        placeholder="Search projects…"
                                        autocomplete="off"
                                        data-bwignore="true"
                                        data-1p-ignore
                                        data-lpignore="true"
                                        class="w-full rounded-[11px] border border-[#e0dbd0] bg-surface px-3.5 py-2.5 text-[14px] outline-none focus:border-accent"
                                        @input="promoteProject = ''"
                                        @keydown.enter.prevent="promoteMatches[0] && pickProject(promoteMatches[0])"
                                    />
                                    <ul v-if="promoteQuery.trim() && !promoteProject" class="mb-4 mt-1.5 max-h-[160px] overflow-y-auto rounded-[11px] border border-[#ebe7de]">
                                        <li
                                            v-for="p in promoteMatches"
                                            :key="p.kendo_id"
                                            class="cursor-pointer px-3.5 py-2 text-[14px] text-ink-soft hover:bg-accent-chip hover:text-accent"
                                            @click="pickProject(p)"
                                        >
                                            {{ p.name }}
                                        </li>
                                        <li v-if="!promoteMatches.length" class="px-3.5 py-2 text-[13px] text-faint-3">No match</li>
                                    </ul>
                                    <div v-else class="mb-4"></div>
                                </template>

                                <!-- estimate is a Kendo-mirror field; drafts have none (ADR-0012) -->
                                <template v-if="item.kind === 'issue'">
                                    <label class="mb-1.5 block font-mono text-[11px] uppercase tracking-[0.08em] text-faint-3">Estimate (hours)</label>
                                    <div class="mb-4 flex items-center gap-2">
                                        <input
                                            v-model="draft.estimate_hours"
                                            type="number"
                                            min="0"
                                            step="0.25"
                                            placeholder="—"
                                            class="min-w-0 flex-1 rounded-[11px] border border-[#e0dbd0] bg-surface px-3.5 py-2.5 text-[14px] outline-none focus:border-accent"
                                        />
                                        <button v-if="draft.estimate_hours !== ''" class="cursor-pointer px-1 text-[18px] leading-none text-faint-2 hover:text-behind" title="Clear" @click="draft.estimate_hours = ''">×</button>
                                    </div>
                                </template>

                                <div class="flex items-center justify-between">
                                    <span v-if="errors[rowKey(item)]" class="font-mono text-[11px] text-behind">{{ errors[rowKey(item)] }}</span>
                                    <span v-else></span>
                                    <div class="flex items-center gap-2">
                                        <button
                                            v-if="item.kind === 'draft' && projects.length"
                                            class="cursor-pointer rounded-[11px] border border-[#e0dbd0] bg-surface px-4 py-2 font-sans text-[13px] font-semibold text-ink-soft transition hover:border-accent hover:text-accent"
                                            title="Create a Kendo issue from this draft"
                                            @click="promote(item)"
                                        >
                                            Promote
                                        </button>
                                        <button
                                            class="cursor-pointer rounded-[11px] bg-accent px-4 py-2 font-sans text-[13px] font-semibold text-white transition hover:bg-accent-deep"
                                            @click="saveEdit(item)"
                                        >
                                            Done
                                        </button>
                                    </div>
                                </div>
                                </Card>
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
                            :disabled="!canStart(item)"
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
            @keydown.esc.window="closePick"
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

    <!-- ad-hoc timing modal: search any Kendo issue, pick it → timer starts (ADR-0015) -->
    <Teleport to="body">
        <div
            v-if="adhocOpen"
            class="fixed inset-0 z-50 flex items-start justify-center bg-black/30 p-4 pt-[15vh]"
            @click.self="adhocOpen = false"
            @keydown.esc.window="adhocOpen = false"
        >
            <div class="w-full max-w-[520px] rounded-[18px] border border-[#ebe7de] bg-surface p-5 shadow-[0_20px_60px_-15px_rgba(42,41,38,0.4)]">
                <div class="mb-1 flex items-center justify-between">
                    <div class="text-[15px] font-semibold tracking-[-0.01em]">Log time on another task</div>
                    <button class="cursor-pointer px-1 text-[18px] leading-none text-faint-2 hover:text-ink-soft" title="Close" @click="adhocOpen = false">×</button>
                </div>
                <div class="mb-3 font-mono text-[11px] text-faint-3">Search any Kendo issue — unassigned PM tasks, reviews of others' tickets. Timer starts on pick.</div>

                <input
                    v-model="adhocQ"
                    type="text"
                    autofocus
                    placeholder="Search Kendo issues…"
                    class="w-full rounded-[10px] border border-[#e0dbd0] bg-white px-3 py-2.5 font-mono text-[13px] outline-none focus:border-accent-tint-2"
                    @input="adhocSearch"
                />

                <div class="mt-3 flex max-h-[320px] flex-col gap-0.5 overflow-auto">
                    <div v-if="adhocLoading" class="px-1 py-2 font-mono text-[12px] text-faint-3">searching…</div>
                    <template v-else>
                        <button
                            v-for="c in adhocResults"
                            :key="c.id"
                            class="cursor-pointer rounded-[8px] px-2 py-2 text-left font-mono text-[12px] text-ink-soft hover:bg-surface-soft hover:text-accent"
                            @click="timeAdhoc(c)"
                        >
                            <span class="font-semibold">{{ c.key }}</span> {{ c.title }}
                        </button>
                        <span v-if="adhocQ && !adhocResults.length" class="px-1 py-2 font-mono text-[12px] text-faint-3">no matches</span>
                    </template>
                </div>
            </div>
        </div>
    </Teleport>
</template>

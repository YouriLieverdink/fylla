<script setup>
// Client board (#56): read-only. A totals band (with run-rate pace) + a filter
// bar + a per-developer subtotal strip over one full-bleed kanban for the whole
// client — columns are the client's real Kendo lanes, cards its issues (colored
// per developer, flagged when overrunning or stuck). Filtering is client-side.
// Reached from a Delivery card.
import { computed, ref } from 'vue';
import { Link } from '@inertiajs/vue3';
import AppHeader from '../Components/AppHeader.vue';
import Card from '../Components/Card.vue';
import Chip from '../Components/Chip.vue';
import EmptyState from '../Components/EmptyState.vue';
import ProgressBar from '../Components/ProgressBar.vue';

const props = defineProps({
    data: { type: Object, required: true },
    history: { type: Object, required: true },
});
const { client, developers, lanes, issues, currentSprintId } = props.data;

// Delivery history (#67): +/− formatting for the per-month and cumulative gaps.
const signed = (n) => `${n > 0 ? '+' : n < 0 ? '−' : '±'}${Math.abs(n)}`;

const STUCK_HELP = 'Stuck = an in-progress issue with no time logged and no lane change in the last 5 working days.';

// Stable per-developer colors (muted, distinct hues); unassigned = grey.
const PALETTE = ['#6b7cff', '#e0873c', '#3fae7d', '#c15b8a', '#5aa0c4', '#9a6bd0', '#b59a3f', '#cf6b52', '#4f9e9e', '#a86b4f'];
const colorMap = Object.fromEntries(developers.map((d, i) => [d.id, PALETTE[i % PALETTE.length]]));
const colorFor = (id) => (id === null ? '#c2bcb0' : (colorMap[id] ?? '#c2bcb0'));

const selectedDevs = ref([]); // empty = all developers
const onlyOver = ref(false);
const onlyStuck = ref(false);
const currentSprint = ref(currentSprintId !== null);

// Everything except the developer filter — drives the subtotal counts so they
// don't all collapse to zero when one developer is selected.
const scoped = computed(() =>
    issues.filter((i) => {
        if (currentSprint.value && currentSprintId !== null && i.sprint !== currentSprintId) return false;
        if (onlyOver.value && !i.over) return false;
        if (onlyStuck.value && !i.stuck) return false;
        return true;
    }),
);
const filtered = computed(() =>
    scoped.value.filter((i) => !selectedDevs.value.length || selectedDevs.value.includes(i.assignee)),
);
const inLane = (lane) => filtered.value.filter((i) => i.lane === lane);
const countFor = (id) => scoped.value.filter((i) => i.assignee === id).length;

const pace = computed(() => {
    if (!client.target || client.projected === null) return null;
    return { projected: client.projected, delta: client.paceDelta, onPace: Math.abs(client.paceDelta) <= client.target * 0.05 };
});
const isSelected = (id) => selectedDevs.value.includes(id);
const toggleDev = (id) =>
    (selectedDevs.value = isSelected(id) ? selectedDevs.value.filter((x) => x !== id) : [...selectedDevs.value, id]);

// Only developers relevant to what's on screen: something in the current view,
// hours logged this month, or currently selected (so a pick never vanishes).
const visibleDevs = computed(() =>
    developers.filter((d) => countFor(d.id) > 0 || d.hoursMonth > 0 || isSelected(d.id)),
);
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[80px] pt-11">
        <AppHeader />

        <!-- header -->
        <div class="mb-3 flex items-end justify-between gap-6">
            <div>
                <Link
                    href="/delivery"
                    class="mb-1 inline-block font-mono text-[11px] font-semibold uppercase tracking-[0.14em] text-faint-2 hover:text-accent"
                >
                    ← Delivery
                </Link>
                <h1 class="text-[34px] font-bold leading-[1.05] tracking-[-0.03em]">{{ client.name }}</h1>
                <p class="mt-1 text-[13px] text-muted">{{ client.meta }}</p>
            </div>
            <Chip v-if="client.sprint" tone="track">
                {{ client.sprint.name }} · {{ client.sprint.done }}/{{ client.sprint.total }} done
            </Chip>
        </div>

        <!-- totals band + delivery history (#67) -->
        <div class="mb-6 flex items-stretch gap-4">
        <Card radius="22px" pad="22px 26px" class="min-w-0 flex-1">
            <div class="grid grid-cols-4 gap-6">
                <div>
                    <div class="mb-1 font-mono text-[10.5px] font-semibold uppercase tracking-[0.12em] text-faint-3">Hours this month</div>
                    <div class="font-mono text-[26px] font-semibold tabular-nums">
                        {{ client.hours }}<span v-if="client.target" class="text-[15px] text-faint-4"> / {{ client.target }}h</span>
                    </div>
                    <ProgressBar v-if="client.target" :value="client.pct" tone="accent" class="mt-2" height="6px" />
                    <div v-if="pace" class="mt-1.5 font-mono text-[10.5px]" :class="pace.onPace ? 'text-track' : 'text-behind'">
                        <template v-if="pace.onPace">on pace · proj. {{ pace.projected }}h</template>
                        <template v-else>proj. {{ pace.projected }}h · {{ pace.delta > 0 ? '+' : '' }}{{ pace.delta }}h vs target</template>
                    </div>
                </div>
                <div>
                    <div class="mb-1 font-mono text-[10.5px] font-semibold uppercase tracking-[0.12em] text-faint-3">Active issues</div>
                    <div class="font-mono text-[26px] font-semibold tabular-nums">{{ client.activeIssues }}</div>
                    <div class="mt-2 text-[12px] text-faint-2">across {{ developers.length }} developers</div>
                </div>
                <div>
                    <div class="mb-1 font-mono text-[10.5px] font-semibold uppercase tracking-[0.12em] text-faint-3">Sprint</div>
                    <template v-if="client.sprint">
                        <div class="text-[18px] font-semibold">{{ client.sprint.name }}</div>
                        <div class="mt-1 text-[12px] text-faint-2">
                            <span v-if="client.sprint.dates">{{ client.sprint.dates }}</span>
                            <span v-if="client.sprint.dates && client.sprint.daysLeft !== null"> · </span>
                            <span v-if="client.sprint.daysLeft !== null">{{ client.sprint.daysLeft }}d left</span>
                        </div>
                    </template>
                    <div v-else class="text-[15px] text-faint-3">No active sprint</div>
                </div>
                <div>
                    <div class="mb-1 font-mono text-[10.5px] font-semibold uppercase tracking-[0.12em] text-faint-3">Flagged</div>
                    <div class="font-mono text-[26px] font-semibold tabular-nums" :class="client.flaggedCount ? 'text-behind' : ''">{{ client.flaggedCount }}</div>
                    <div class="mt-2 text-[12px] text-faint-2">{{ client.overrunningCount }} overrunning · {{ client.stuckCount }} stuck</div>
                </div>
            </div>
        </Card>

        <!-- delivery history: delivered vs target, last few months -->
        <Card radius="22px" pad="20px 24px" class="w-[300px] flex-none">
            <h2 class="mb-2 font-mono text-[10.5px] font-semibold uppercase tracking-[0.12em] text-faint-3">Delivery history</h2>
            <div
                v-for="row in history.rows"
                :key="row.month"
                class="flex items-baseline justify-between border-b border-card-border py-1 last:border-b-0"
            >
                <span class="font-mono text-[11.5px]" :class="row.current ? 'text-ink' : 'text-muted'">
                    {{ row.month }}<span v-if="row.current" class="text-faint-3"> · so far</span>
                </span>
                <span class="font-mono text-[12px] tabular-nums">
                    {{ row.delivered }}<span v-if="row.target !== null" class="text-faint-4"> / {{ row.target }}</span>h
                    <span
                        v-if="row.delta !== null && !row.current"
                        class="ml-1.5 text-[11px]"
                        :class="row.delta < 0 ? 'text-behind' : 'text-track'"
                    >{{ signed(row.delta) }}h</span>
                </span>
            </div>
            <div v-if="history.gap !== null" class="mt-2 flex items-baseline justify-between">
                <span class="text-[11px] text-faint-2" title="Completed months only — the current month is where this gets spent">Cumulative (completed)</span>
                <span class="font-mono text-[12px] font-semibold tabular-nums" :class="history.gap < 0 ? 'text-behind' : 'text-track'">
                    {{ signed(history.gap) }}h
                </span>
            </div>
        </Card>
        </div>

        <!-- filter bar -->
        <div class="mb-3 flex flex-wrap items-center gap-2">
            <button
                v-if="currentSprintId !== null"
                type="button"
                class="rounded-full border px-3 py-1.5 font-mono text-[12px] font-medium transition"
                :class="currentSprint ? 'border-transparent bg-accent-tint text-accent-deep' : 'border-card-border text-muted hover:bg-surface-soft'"
                @click="currentSprint = !currentSprint"
            >
                Current sprint
            </button>
            <button
                type="button"
                class="rounded-full border px-3 py-1.5 font-mono text-[12px] font-medium transition"
                :class="onlyOver ? 'border-transparent bg-behind-tint text-behind' : 'border-card-border text-muted hover:bg-surface-soft'"
                @click="onlyOver = !onlyOver"
            >
                Over estimate
            </button>
            <button
                type="button"
                :title="STUCK_HELP"
                class="rounded-full border px-3 py-1.5 font-mono text-[12px] font-medium transition"
                :class="onlyStuck ? 'border-transparent bg-behind-tint text-behind' : 'border-card-border text-muted hover:bg-surface-soft'"
                @click="onlyStuck = !onlyStuck"
            >
                Stuck
                <span class="ml-1 text-faint-3">ⓘ</span>
            </button>

            <span class="ml-auto font-mono text-[11px] text-faint-3">{{ filtered.length }} issues</span>
        </div>

        <!-- board -->
        <Card
            v-if="lanes.length"
            radius="20px"
            pad="16px 18px"
            class="overflow-x-auto"
        >
            <div class="flex h-[calc(100vh-300px)] min-h-[420px] gap-3">
                <div v-for="lane in lanes" :key="lane.name" class="flex w-[264px] flex-none flex-col">
                    <div class="mb-2 flex items-center justify-between px-1">
                        <span class="font-mono text-[11px] font-semibold uppercase tracking-[0.1em] text-faint-3">{{ lane.name }}</span>
                        <span class="font-mono text-[11px] tabular-nums text-faint-4">{{ inLane(lane.name).length }}</span>
                    </div>
                    <div class="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto rounded-[14px] bg-surface-soft p-2">
                        <a
                            v-for="i in inLane(lane.name)"
                            :key="i.key"
                            :href="i.kendo_url"
                            target="_blank"
                            class="block rounded-[12px] border border-card-border border-l-4 px-3 py-2.5 shadow-[0_1px_2px_rgba(42,41,38,0.04)] transition hover:shadow-[0_2px_8px_rgba(42,41,38,0.12)]"
                            :class="i.over ? 'bg-[#fdf3f2]' : i.stuck ? 'bg-[#fdf8ec]' : 'bg-surface'"
                            :style="{ borderLeftColor: colorFor(i.assignee) }"
                        >
                            <div class="line-clamp-2 text-[13px] font-medium leading-snug">{{ i.title }}</div>
                            <div class="mt-1.5 flex items-center justify-between gap-2">
                                <span class="truncate font-mono text-[10px] text-faint-3">{{ i.key }}</span>
                                <span class="flex-none font-mono text-[10px] tabular-nums text-faint-3">
                                    {{ i.loggedHours.toFixed(1) }}<span v-if="i.estimateHours !== null">/{{ i.estimateHours.toFixed(1) }}</span>h
                                </span>
                            </div>
                            <div class="mt-2 flex flex-wrap items-center gap-1.5">
                                <span class="flex min-w-0 items-center gap-1.5">
                                    <span class="h-2 w-2 flex-none rounded-full" :style="{ background: colorFor(i.assignee) }"></span>
                                    <span class="truncate font-mono text-[10px] text-[#8a8578]">{{ i.assigneeName }}</span>
                                </span>
                                <Chip v-if="i.over" tone="behind">+{{ i.overPct }}%</Chip>
                                <Chip v-if="i.stuck" tone="neutral" :title="STUCK_HELP">stuck<span v-if="i.idleDays !== null"> · {{ i.idleDays }}d</span></Chip>
                            </div>
                        </a>
                    </div>
                </div>
            </div>
        </Card>

        <EmptyState
            v-else
            title="Nothing matches"
            text="No issues match the current filters. Try clearing a filter."
        />

        <!-- per-developer subtotals (also a color legend + quick filter) -->
        <div class="mt-4 flex flex-wrap gap-2">
            <button
                v-for="d in visibleDevs"
                :key="d.id"
                type="button"
                class="flex items-center gap-2 rounded-full border px-2.5 py-1 font-mono text-[11px] transition"
                :class="isSelected(d.id) ? 'border-accent bg-surface-soft' : 'border-card-border hover:bg-surface-soft'"
                @click="toggleDev(d.id)"
            >
                <span class="h-2.5 w-2.5 flex-none rounded-full" :style="{ background: colorFor(d.id) }"></span>
                <span class="font-medium text-ink">{{ d.name }}</span>
                <span class="tabular-nums text-faint-3">{{ countFor(d.id) }} · {{ d.hoursMonth.toFixed(0) }}h/mo</span>
            </button>
        </div>
    </div>
</template>

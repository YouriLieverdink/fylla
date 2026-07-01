<script setup>
import { ref } from 'vue';
import AppButton from '../Components/AppButton.vue';
import Chip from '../Components/Chip.vue';
import Card from '../Components/Card.vue';
import SunkenPanel from '../Components/SunkenPanel.vue';
import SegmentedControl from '../Components/SegmentedControl.vue';
import TabBar from '../Components/TabBar.vue';
import SyncStatus from '../Components/SyncStatus.vue';
import EmptyState from '../Components/EmptyState.vue';
import BillableMetric from '../Components/BillableMetric.vue';
import TimerStack from '../Components/TimerStack.vue';
import UtilizationTrendChart from '../Components/UtilizationTrendChart.vue';
import SprintPacingChart from '../Components/SprintPacingChart.vue';
import WorkItemsTable from '../Components/WorkItemsTable.vue';
import EstimateBias from '../Components/EstimateBias.vue';
import ClientCard from '../Components/ClientCard.vue';
import TeamList from '../Components/TeamList.vue';

const lens = ref('Personal');
const tab = ref('Overview');

const surfaces = [
    { name: 'Canvas', hex: '#F4F2EE', cls: 'bg-canvas border border-border' },
    { name: 'Surface', hex: '#FFFFFF', cls: 'bg-surface border border-border' },
    { name: 'Sunken', hex: '#EDEAE3', cls: 'bg-sunken border border-border' },
    { name: 'Border', hex: '#E6E2D9', cls: 'bg-border' },
    { name: 'Ink', hex: '#2B2A27', cls: 'bg-ink' },
    { name: 'Muted', hex: '#6D6A63', cls: 'bg-muted' },
    { name: 'On track', hex: '#5C8A6F', cls: 'bg-track' },
    { name: 'Behind', hex: '#B18749', cls: 'bg-behind' },
];
const accentRamp = ['bg-accent-tint', 'bg-accent-tint-2', 'bg-accent', 'bg-accent-deep'];

const paused = [
    { key: 'FYL-228', title: 'Review sync webhook retries', time: '00:42:15' },
    { key: 'FYL-215', title: 'Standup & sprint planning', time: '01:05:40' },
];

const workItems = [
    { key: 'FYL-231', title: 'Refactor invoice PDF export', type: 'Feature', billable: true, est: 6.0, act: 4.5, actTone: 'behind', priority: 'High' },
    { key: 'FYL-228', title: 'Review sync webhook retries', type: 'Bug', billable: true, est: 3.0, act: 2.5, actTone: 'track', priority: 'Med' },
    { key: 'FYL-224', title: 'Client onboarding email flow', type: 'Feature', billable: true, est: 8.0, act: 11.5, actTone: 'behind', priority: 'High', highlight: true },
    { key: 'FYL-219', title: 'Internal: upgrade CI runners', type: 'Chore', billable: false, est: 2.0, act: 2.0, actTone: 'muted', priority: 'Low' },
];

const team = [
    { initials: 'SR', name: 'Sofia Reyes', hours: '34 → 32h', pct: 94, tone: 'track', avatarTone: 'accent', aging: '2d', agingTone: 'neutral' },
    { initials: 'DK', name: 'David Kwon', hours: '28 → 35h', pct: 78, tone: 'behind', avatarTone: 'track', aging: '6d', agingTone: 'behind' },
    { initials: 'MA', name: 'Maya Abdi', hours: '40 → 38h', pct: 88, tone: 'track', avatarTone: 'behind', aging: '1d', agingTone: 'neutral' },
    { initials: 'TA', name: 'Tom Ashby', hours: '22 → 20h', pct: 64, tone: 'track', avatarTone: 'neutral', aging: '3d', agingTone: 'neutral' },
];
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-[60px]">
        <!-- header -->
        <header class="mb-[52px] flex items-end justify-between gap-6 border-b border-border pb-[34px]">
            <div>
                <div class="mb-4 flex items-center gap-3">
                    <div class="relative h-[34px] w-[34px] rounded-[11px] bg-accent shadow-[0_5px_15px_-5px_rgba(108,95,201,0.6)]">
                        <div class="absolute inset-0 flex items-center justify-center">
                            <div
                                class="h-3 w-3 rounded-full border-[2.5px] border-white border-t-transparent"
                                style="transform: rotate(35deg)"
                            ></div>
                        </div>
                    </div>
                    <span class="text-[22px] font-semibold tracking-[-0.02em]">Fylla</span>
                </div>
                <h1 class="max-w-[15ch] text-[41px] font-bold leading-[1.05] tracking-[-0.03em]">The calm instrument</h1>
                <p class="mt-[15px] max-w-[54ch] text-[16px] leading-[1.55] text-muted">
                    Design tokens and a component library for a single-user time-tracking command center. Numbers
                    speak; chrome recedes.
                </p>
            </div>
            <div class="flex-none text-right">
                <div class="mb-2 font-mono text-[10.5px] font-semibold uppercase tracking-[0.14em] text-faint">Design system</div>
                <div class="font-mono text-[13px] font-medium text-muted">v0.1 · Light</div>
            </div>
        </header>

        <!-- FOUNDATIONS -->
        <section class="mb-[66px]">
            <div class="mb-[26px] font-mono text-[12px] font-semibold uppercase tracking-[0.16em] text-faint-2">Foundations</div>
            <div class="grid grid-cols-2 items-start gap-[22px]">
                <Card>
                    <div class="mb-[22px] font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">Signature accent</div>
                    <div class="mb-5 flex items-center gap-5">
                        <div class="h-[92px] w-[92px] flex-none rounded-[22px] bg-accent shadow-[0_12px_28px_-10px_rgba(108,95,201,0.6)]"></div>
                        <div>
                            <div class="mb-[5px] text-[16px] font-semibold">Periwinkle</div>
                            <div class="font-mono text-[13px] font-medium leading-[1.7] text-muted">#6C5FC9<br />oklch(.55 .14 285)</div>
                            <div class="mt-[9px] max-w-[26ch] text-[12.5px] leading-[1.5] text-faint">
                                Reserved almost exclusively for the billable metric — the one number that matters.
                            </div>
                        </div>
                    </div>
                    <div class="flex gap-2.5">
                        <div v-for="c in accentRamp" :key="c" class="h-8 flex-1 rounded-[10px]" :class="c"></div>
                    </div>
                </Card>

                <Card>
                    <div class="mb-[22px] font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">Surfaces &amp; text</div>
                    <div class="grid grid-cols-4 gap-x-3 gap-y-4">
                        <div v-for="s in surfaces" :key="s.name">
                            <div class="h-[50px] rounded-[12px]" :class="s.cls"></div>
                            <div class="mt-[7px] text-[11.5px] font-medium">{{ s.name }}</div>
                            <div class="mt-[3px] font-mono text-[10px] text-faint-2">{{ s.hex }}</div>
                        </div>
                    </div>
                </Card>

                <Card>
                    <div class="mb-[22px] font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">Type — Hanken Grotesk</div>
                    <div class="flex flex-col gap-[15px]">
                        <div class="flex items-baseline justify-between gap-4 border-b border-divider pb-[13px]">
                            <span class="text-[37px] font-bold tracking-[-0.03em]">Display</span>
                            <span class="font-mono text-[10px] text-faint-2">42 / 700</span>
                        </div>
                        <div class="flex items-baseline justify-between gap-4 border-b border-divider pb-[13px]">
                            <span class="text-[22px] font-semibold tracking-[-0.01em]">Heading</span>
                            <span class="font-mono text-[10px] text-faint-2">22 / 600</span>
                        </div>
                        <div class="flex items-baseline justify-between gap-4 border-b border-divider pb-[13px]">
                            <span class="text-[15px] text-ink-soft">Body — quiet hierarchy, generous line height.</span>
                            <span class="flex-none font-mono text-[10px] text-faint-2">15 / 400</span>
                        </div>
                        <div class="flex items-baseline justify-between gap-4">
                            <span class="font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">Label / Mono caps</span>
                            <span class="font-mono text-[10px] text-faint-2">11 / .13em</span>
                        </div>
                    </div>
                </Card>

                <Card>
                    <div class="mb-[22px] font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">Spline Sans Mono — figures</div>
                    <div class="mb-1.5 font-mono text-[34px] font-semibold tabular-nums tracking-[-0.01em]">73.8<span class="text-[20px] text-faint-2">%</span></div>
                    <div class="font-mono text-[15px] font-medium leading-[1.9] tabular-nums text-muted">01:47:32 · elapsed<br />FYL-231 · 4.5h / 6.0h<br />0123456789 tabular</div>
                    <div class="mt-5 flex gap-[22px] border-t border-divider pt-[18px]">
                        <div>
                            <div class="mb-2 font-mono text-[10px] text-faint-2">RADIUS</div>
                            <div class="flex items-end gap-2">
                                <div class="h-[26px] w-[26px] rounded-[8px] border border-border bg-sunken"></div>
                                <div class="h-[30px] w-[30px] rounded-[12px] border border-border bg-sunken"></div>
                                <div class="h-[34px] w-[34px] rounded-[20px] border border-border bg-sunken"></div>
                                <div class="h-[34px] w-[34px] rounded-full border border-border bg-sunken"></div>
                            </div>
                            <div class="mt-[7px] font-mono text-[9.5px] text-faint-2">8 · 12 · 20 · full</div>
                        </div>
                        <div class="flex-1">
                            <div class="mb-2 font-mono text-[10px] text-faint-2">SPACE · 4pt</div>
                            <div class="flex h-[34px] items-end gap-1.5">
                                <div class="h-2 w-1 rounded-sm bg-accent-tint-2"></div>
                                <div class="h-3.5 w-2 rounded-sm bg-accent-tint-2"></div>
                                <div class="h-5 w-3 rounded-sm bg-accent-tint-2"></div>
                                <div class="h-[26px] w-4 rounded-sm bg-accent-tint-2"></div>
                                <div class="h-[34px] w-6 rounded-sm bg-accent"></div>
                            </div>
                            <div class="mt-[7px] font-mono text-[9.5px] text-faint-2">4 8 12 16 24 32</div>
                        </div>
                    </div>
                </Card>
            </div>
        </section>

        <!-- PERSONAL LENS -->
        <section class="mb-10">
            <div class="mb-2 font-mono text-[12px] font-semibold uppercase tracking-[0.16em] text-faint-2">Personal lens · the number that matters</div>
            <p class="mb-[26px] max-w-[60ch] text-[14px] text-faint">
                Billable share of contracted hours, as a soft signal against a 75% target — a rolling trend, never a
                pass/fail verdict.
            </p>
            <div class="grid grid-cols-[400px_1fr] items-stretch gap-[22px]">
                <BillableMetric />
                <UtilizationTrendChart />
            </div>
        </section>

        <section class="mb-10">
            <div class="grid grid-cols-[400px_1fr] items-stretch gap-[22px]">
                <TimerStack
                    :active="{ key: 'FYL-231', title: 'Refactor invoice PDF export', time: '01:47:32' }"
                    :paused="paused"
                    hint="stop active → FYL-228 resumes"
                />
                <SprintPacingChart />
            </div>
        </section>

        <!-- PM LENS -->
        <section class="mb-10">
            <div class="mb-2 font-mono text-[12px] font-semibold uppercase tracking-[0.16em] text-faint-2">Project-manager lens · read-only</div>
            <p class="mb-[26px] max-w-[60ch] text-[14px] text-faint">
                A quiet overview across clients and developers. Work items, estimate bias, client targets, teammate
                pacing.
            </p>
            <div class="grid grid-cols-[1fr_380px] items-start gap-[22px]">
                <WorkItemsTable :items="workItems" />
                <EstimateBias />
            </div>
        </section>

        <section class="mb-10">
            <div class="grid grid-cols-2 items-start gap-[22px]">
                <div class="flex flex-col gap-4">
                    <div class="font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">Clients · monthly hour targets</div>
                    <ClientCard
                        initials="Me"
                        name="Meridian Studio"
                        meta="Retainer · 4 developers"
                        hours="128"
                        target="160"
                        :pct="80"
                        tone="track"
                        status="80% · on pace"
                        days-left="9 working days left"
                    />
                    <ClientCard
                        initials="No"
                        name="Northwind Labs"
                        meta="Project · 2 developers"
                        hours="41"
                        target="80"
                        :pct="51"
                        tone="behind"
                        status="51% · slightly behind"
                        days-left="9 working days left"
                    />
                </div>
                <TeamList :members="team" />
            </div>
        </section>

        <!-- PRIMITIVES -->
        <section>
            <div class="mb-[26px] font-mono text-[12px] font-semibold uppercase tracking-[0.16em] text-faint-2">Primitives</div>
            <div class="grid grid-cols-2 items-start gap-[22px]">
                <div class="flex flex-col gap-[22px]">
                    <Card>
                        <div class="mb-[18px] font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">Buttons</div>
                        <div class="flex flex-wrap items-center gap-3">
                            <AppButton variant="primary">Primary</AppButton>
                            <AppButton variant="secondary">Secondary</AppButton>
                            <AppButton variant="ghost">Ghost</AppButton>
                            <AppButton variant="disabled">Disabled</AppButton>
                            <AppButton variant="secondary" size="sm" dot>Start timer</AppButton>
                        </div>
                    </Card>

                    <Card>
                        <div class="mb-[18px] font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">View switcher</div>
                        <SegmentedControl v-model="lens" :options="['Personal', 'Project manager']" />
                        <div class="mt-[22px]">
                            <TabBar v-model="tab" :tabs="['Overview', 'Timesheet', 'Clients']" />
                        </div>
                    </Card>

                    <SyncStatus last-synced="last synced 3 min ago · 14:32" />
                </div>

                <div class="flex flex-col gap-[22px]">
                    <Card>
                        <div class="mb-[18px] font-mono text-[11px] font-semibold uppercase tracking-[0.13em] text-faint">Panels &amp; chips</div>
                        <div class="mb-5 flex flex-wrap gap-[9px]">
                            <Chip tone="billable">billable</Chip>
                            <Chip tone="neutral">non-billable</Chip>
                            <Chip tone="track">on track</Chip>
                            <Chip tone="behind">behind</Chip>
                        </div>
                        <SunkenPanel title="Sunken panel">
                            <div class="text-[12.5px] leading-[1.5] text-faint-2">Nested surface for grouping — a step quieter than the base card.</div>
                        </SunkenPanel>
                    </Card>

                    <EmptyState
                        title="No timers running"
                        text="Start tracking against a work item and it'll appear here. Stack a second one and it pushes on top."
                    >
                        <template #action>
                            <AppButton variant="primary" size="sm">Start a timer</AppButton>
                        </template>
                    </EmptyState>
                </div>
            </div>
        </section>
    </div>
</template>

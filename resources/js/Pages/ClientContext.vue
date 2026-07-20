<script setup>
// Client context (#56): read-only, single-column report over the synced_issues
// mirror + developer roster, scoped to one managed client. Variant A layout
// (#57): brief stat-band → per-developer estimate-vs-actual table → two
// side-by-side attention panels. Reached from a Delivery card.
import { Link } from '@inertiajs/vue3';
import AppHeader from '../Components/AppHeader.vue';
import Card from '../Components/Card.vue';
import Chip from '../Components/Chip.vue';
import EmptyState from '../Components/EmptyState.vue';
import ProgressBar from '../Components/ProgressBar.vue';

const props = defineProps({ data: { type: Object, required: true } });
const { client, developers, overrunning, aging, devById } = props.data;

const needsAttention = client.overrunningCount + client.agingCount;

// Bias marker: 0% sits centre, clamped so extremes stay on the track.
const biasPos = (pct) => Math.max(4, Math.min(96, 50 + pct * 0.9));
const onTarget = (pct) => Math.abs(pct) <= 15;
const sign = (n) => (n > 0 ? '+' : '') + n + '%';
const devName = (id) => devById[id]?.name ?? 'Unassigned';
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[140px] pt-11">
        <AppHeader />

        <!-- client brief: hero band -->
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

        <Card radius="22px" pad="22px 26px" class="mb-9">
            <div class="grid grid-cols-4 gap-6">
                <div>
                    <div class="mb-1 font-mono text-[10.5px] font-semibold uppercase tracking-[0.12em] text-faint-3">Hours this month</div>
                    <div class="font-mono text-[26px] font-semibold tabular-nums">
                        {{ client.hours }}<span v-if="client.target" class="text-[15px] text-faint-4"> / {{ client.target }}h</span>
                    </div>
                    <ProgressBar v-if="client.target" :value="client.pct" tone="accent" class="mt-2" height="6px" />
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
                    <div class="mb-1 font-mono text-[10.5px] font-semibold uppercase tracking-[0.12em] text-faint-3">Needs attention</div>
                    <div class="font-mono text-[26px] font-semibold tabular-nums" :class="needsAttention ? 'text-behind' : ''">{{ needsAttention }}</div>
                    <div class="mt-2 text-[12px] text-faint-2">{{ client.overrunningCount }} overrunning · {{ client.agingCount }} aging</div>
                </div>
            </div>
        </Card>

        <!-- estimate vs actual: full-width per-developer table -->
        <div class="mb-3 flex items-center justify-between">
            <div class="text-[16px] font-semibold tracking-[-0.01em]">Estimate vs actual</div>
            <div class="font-mono text-[11px] text-faint-3">per developer · rolling last 20</div>
        </div>
        <Card radius="24px" pad="8px 10px 12px" class="mb-9">
            <div v-if="developers.length" class="grid grid-cols-[1fr_120px_120px_1fr_80px] gap-3 px-5 py-3 font-mono text-[10px] font-semibold uppercase tracking-[0.1em] text-faint-3">
                <span>Developer</span><span class="text-right">Median est</span><span class="text-right">Median act</span><span class="px-4">Bias (under → over)</span><span class="text-right">Within ±15%</span>
            </div>
            <div
                v-for="d in developers"
                :key="d.id"
                class="grid grid-cols-[1fr_120px_120px_1fr_80px] items-center gap-3 border-t border-divider-soft px-5 py-4"
            >
                <span class="text-[14px] font-semibold" :class="!d.hasData && 'text-muted'">{{ d.name }}</span>
                <template v-if="d.hasData">
                    <span class="text-right font-mono text-[13px] tabular-nums text-muted">{{ d.medianEst.toFixed(1) }}h</span>
                    <span class="text-right font-mono text-[13px] tabular-nums text-muted">{{ d.medianActual.toFixed(1) }}h</span>
                    <div class="px-4">
                        <div class="relative h-1.5 rounded-full" style="background: linear-gradient(90deg, #e6e2d9, #edeae3, #e6e2d9)">
                            <div class="absolute -top-[3px] left-1/2 h-3 w-px -translate-x-1/2 bg-[#cbc6ba]"></div>
                            <div
                                class="absolute -top-1 h-3.5 w-3.5 -translate-x-1/2 rounded-full border-[2.5px] border-white shadow-[0_2px_6px_rgba(42,41,38,0.15)]"
                                :class="onTarget(d.biasPct) ? 'bg-track' : 'bg-behind'"
                                :style="{ left: biasPos(d.biasPct) + '%' }"
                            ></div>
                        </div>
                        <div class="mt-1.5 text-center font-mono text-[11px] font-medium" :class="onTarget(d.biasPct) ? 'text-track' : 'text-behind'">{{ sign(d.biasPct) }}</div>
                    </div>
                    <span class="text-right font-mono text-[13px] tabular-nums" :class="d.withinPct >= 50 ? 'text-track' : 'text-behind'">{{ d.withinPct }}%</span>
                </template>
                <span v-else class="col-span-4 font-mono text-[12px] text-faint-3">No completed estimates yet</span>
            </div>
            <EmptyState
                v-if="!developers.length"
                title="No completed work yet"
                text="Once developers finish estimated issues on this client's projects, their estimate-vs-actual bias shows up here."
            />
        </Card>

        <!-- attention: two side-by-side panels -->
        <div class="grid grid-cols-2 items-start gap-[22px]">
            <Card radius="24px" pad="8px 10px 12px">
                <div class="flex items-center justify-between px-5 pb-2 pt-4">
                    <div class="text-[16px] font-semibold tracking-[-0.01em]">Overrunning now</div>
                    <Chip tone="behind">{{ overrunning.length }} · logged &gt; est</Chip>
                </div>
                <div v-for="i in overrunning" :key="i.key" class="border-t border-divider-soft px-5 py-3.5">
                    <div class="flex items-center justify-between">
                        <span class="truncate text-[14px] font-medium">{{ i.title }}</span>
                        <span class="ml-3 flex-none font-mono text-[13px] font-semibold tabular-nums text-behind">+{{ i.overPct }}%</span>
                    </div>
                    <div class="mt-1 font-mono text-[11px] text-faint-3">{{ i.key }} · {{ devName(i.assignee) }} · {{ i.est.toFixed(1) }} → {{ i.logged.toFixed(1) }}h</div>
                </div>
                <div v-if="!overrunning.length" class="px-5 py-6 text-center text-[13px] text-faint-3">Nothing overrunning.</div>
            </Card>

            <Card radius="24px" pad="8px 10px 12px">
                <div class="flex items-center justify-between px-5 pb-2 pt-4">
                    <div class="text-[16px] font-semibold tracking-[-0.01em]">In-progress aging</div>
                    <Chip tone="neutral">by time in lane</Chip>
                </div>
                <div v-for="i in aging" :key="i.key" class="border-t border-divider-soft px-5 py-3.5">
                    <div class="flex items-center justify-between">
                        <span class="truncate text-[14px] font-medium">{{ i.title }}</span>
                        <span class="ml-3 flex-none font-mono text-[13px] font-semibold tabular-nums" :class="i.days >= 5 ? 'text-behind' : 'text-muted'">{{ i.days ?? '–' }}d</span>
                    </div>
                    <div class="mt-1 font-mono text-[11px] text-faint-3">{{ i.key }} · {{ devName(i.assignee) }}<span v-if="i.lane"> · {{ i.lane }}</span></div>
                </div>
                <div v-if="!aging.length" class="px-5 py-6 text-center text-[13px] text-faint-3">Nothing aging.</div>
            </Card>
        </div>
    </div>
</template>

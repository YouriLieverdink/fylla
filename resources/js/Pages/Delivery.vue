<script setup>
import { router } from '@inertiajs/vue3';
import AppHeader from '../Components/AppHeader.vue';
import DeliveryProjectionChart from '../Components/DeliveryProjectionChart.vue';
import EmptyState from '../Components/EmptyState.vue';
import { usePageCursor } from '../Composables/usePageCursor';

const props = defineProps({
    clients: { type: Array, default: () => [] },
    projects: { type: Array, default: () => [] },
});

// j/k cursor over the client projection cards, row-major (#43).
const cursor = usePageCursor(() => props.clients.map((c) => 'd-' + c.id));

// A client's assigned projects — drives its billable pills.
const assigned = (clientId) => props.projects.filter((p) => p.client_id === clientId);

// Footer edits hit the existing write routes (no new endpoints, #62).
function toggleBillable(project) {
    router.patch('/projects/' + project.id, { billable: !project.billable }, { preserveScroll: true });
}

function setTarget(card, value) {
    router.patch(
        '/clients/' + card.id,
        { monthly_target_hours: value === '' ? null : Number(value) },
        { preserveScroll: true },
    );
}
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-11">
        <AppHeader />

        <div class="mb-8">
            <h1 class="mb-3 text-[34px] font-bold leading-[1.05] tracking-[-0.03em]">Delivery</h1>
            <p class="max-w-[62ch] text-[15px] leading-[1.55] text-muted">
                Team-aggregate hours delivered this month per client — every developer's worklogs plus
                your own, billable and non-billable, against each client's monthly target. Set targets
                and flag billable inline; clients without a target show delivered hours alone.
            </p>
        </div>

        <div v-if="clients.length" class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <DeliveryProjectionChart
                v-for="c in clients"
                :key="c.id"
                :href="`/delivery/${c.id}`"
                :data-row="'d-' + c.id"
                class="scroll-my-12"
                :class="cursor.isActive('d-' + c.id) && 'ring-2 ring-accent'"
                :initials="c.initials"
                :name="c.name"
                :meta="c.meta"
                :hours="c.hours"
                :target="c.target"
                :projected="c.projected"
                :over-under="c.overUnder"
                :series="c.series"
                :today="c.today"
                :days-in-month="c.daysInMonth"
                :days-left="c.daysLeft"
            >
                <!-- Config footer strip: never navigates (#62). -->
                <template #footer>
                    <div class="flex flex-wrap items-center gap-x-4 gap-y-2.5 border-t border-divider px-[30px] py-3.5">
                        <label class="flex items-center gap-1.5 font-mono text-[11px] uppercase tracking-[0.1em] text-faint-3">
                            <input
                                type="number"
                                min="0"
                                step="1"
                                :value="c.target ?? ''"
                                placeholder="—"
                                class="w-[64px] rounded-[9px] border bg-surface px-2 py-1 text-right font-sans text-[13px] tabular-nums outline-none focus:border-accent"
                                :class="c.target === null ? 'border-accent' : 'border-[#e0dbd0]'"
                                @change="setTarget(c, $event.target.value)"
                            />
                            h/mo
                        </label>

                        <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
                            <button
                                v-for="p in assigned(c.id)"
                                :key="p.id"
                                type="button"
                                class="inline-flex cursor-pointer items-center gap-1.5 rounded-full border px-2.5 py-1 text-[12px] transition"
                                :class="
                                    p.billable
                                        ? 'border-accent-tint-2 bg-accent-tint text-accent-deep'
                                        : 'border-divider text-faint-3 hover:bg-canvas'
                                "
                                @click="toggleBillable(p)"
                            >
                                <span class="h-1.5 w-1.5 rounded-full" :class="p.billable ? 'bg-accent' : 'bg-faint-3'"></span>
                                {{ p.name }}
                            </button>
                        </div>

                        <div class="flex items-center gap-3">
                            <button type="button" class="cursor-pointer font-mono text-[11px] font-semibold uppercase tracking-[0.1em] text-faint-3 transition hover:text-ink">
                                + project
                            </button>
                            <button type="button" class="cursor-pointer font-mono text-[11px] font-semibold uppercase tracking-[0.1em] text-faint-3 transition hover:text-behind">
                                Delete
                            </button>
                        </div>
                    </div>
                </template>
            </DeliveryProjectionChart>
        </div>
        <EmptyState
            v-else
            title="No clients yet"
            text="Create a client on the Clients page and assign it Kendo projects — its team's monthly hours will show up here."
        />
    </div>
</template>

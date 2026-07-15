<script setup>
import AppHeader from '../Components/AppHeader.vue';
import DeliveryProjectionChart from '../Components/DeliveryProjectionChart.vue';
import EmptyState from '../Components/EmptyState.vue';

defineProps({
    clients: { type: Array, default: () => [] },
});
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-11">
        <AppHeader />

        <div class="mb-8">
            <h1 class="mb-3 text-[34px] font-bold leading-[1.05] tracking-[-0.03em]">Delivery</h1>
            <p class="max-w-[62ch] text-[15px] leading-[1.55] text-muted">
                Team-aggregate hours delivered this month per client — every developer's worklogs plus
                your own, billable and non-billable, against each client's monthly target. Clients
                without a target show delivered hours alone.
            </p>
        </div>

        <div v-if="clients.length" class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <DeliveryProjectionChart
                v-for="c in clients"
                :key="c.id"
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
            />
        </div>
        <EmptyState
            v-else
            title="No clients yet"
            text="Create a client on the Clients page and assign it Kendo projects — its team's monthly hours will show up here."
        />
    </div>
</template>

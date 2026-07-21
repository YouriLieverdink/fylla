<script setup>
import { ref, watch } from 'vue';
import { router } from '@inertiajs/vue3';
import AppHeader from '../Components/AppHeader.vue';
import Card from '../Components/Card.vue';
import EmptyState from '../Components/EmptyState.vue';

const props = defineProps({
    rows: { type: Array, default: () => [] },
    total: { type: Number, default: 0 },
    filters: { type: Object, default: () => ({}) },
    clients: { type: Array, default: () => [] },
    projects: { type: Array, default: () => [] },
    developers: { type: Array, default: () => [] },
});

// Local copy of the server-echoed filters; every change reloads via the URL so
// searches are shareable/bookmarkable. Debounced for the free-text input.
const form = ref({ ...props.filters });

let timer;
watch(
    form,
    () => {
        clearTimeout(timer);
        timer = setTimeout(apply, 300);
    },
    { deep: true },
);

function apply() {
    const query = Object.fromEntries(Object.entries(form.value).filter(([, v]) => v));
    router.get('/notes', query, { preserveState: true, preserveScroll: true });
}

function hours(minutes) {
    return (minutes / 60).toFixed(1) + 'h';
}
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-11">
        <AppHeader />

        <div class="mb-8">
            <h1 class="mb-3 text-[34px] font-bold leading-[1.05] tracking-[-0.03em]">Notes</h1>
            <p class="max-w-[62ch] text-[15px] leading-[1.55] text-muted">
                Every synced worklog note, newest first — yours everywhere, teammates' on managed-client
                projects. Search matches the note text and the issue key/title.
            </p>
        </div>

        <div class="mb-6 flex flex-wrap items-center gap-2.5">
            <input
                v-model="form.q"
                type="search"
                placeholder="Search notes…"
                class="w-[280px] rounded-xl border border-card-border bg-surface px-4 py-2 text-[13px] outline-none transition focus:border-accent"
            />
            <select
                v-model="form.client"
                class="rounded-xl border border-card-border bg-surface px-3 py-2 text-[13px] outline-none"
                :class="form.client ? 'text-ink' : 'text-faint'"
            >
                <option :value="null">All clients</option>
                <option v-for="c in clients" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select>
            <select
                v-model="form.project"
                class="rounded-xl border border-card-border bg-surface px-3 py-2 text-[13px] outline-none"
                :class="form.project ? 'text-ink' : 'text-faint'"
            >
                <option :value="null">All projects</option>
                <option v-for="p in projects" :key="p.kendo_id" :value="p.kendo_id">{{ p.name }}</option>
            </select>
            <select
                v-model="form.developer"
                class="rounded-xl border border-card-border bg-surface px-3 py-2 text-[13px] outline-none"
                :class="form.developer ? 'text-ink' : 'text-faint'"
            >
                <option :value="null">All developers</option>
                <option v-for="d in developers" :key="d.kendo_id" :value="d.kendo_id">{{ d.name }}</option>
            </select>
            <input
                v-model="form.from"
                type="date"
                class="rounded-xl border border-card-border bg-surface px-3 py-2 text-[13px] outline-none"
                :class="form.from ? 'text-ink' : 'text-faint'"
            />
            <span class="text-[12px] text-faint-3">to</span>
            <input
                v-model="form.to"
                type="date"
                class="rounded-xl border border-card-border bg-surface px-3 py-2 text-[13px] outline-none"
                :class="form.to ? 'text-ink' : 'text-faint'"
            />
            <span class="ml-auto font-mono text-[11px] uppercase tracking-[0.08em] text-faint-3">
                {{ total }} {{ total === 1 ? 'note' : 'notes' }}{{ total > rows.length ? ` · showing ${rows.length}` : '' }}
            </span>
        </div>

        <Card v-if="rows.length" pad="8px 0">
            <table class="w-full border-collapse text-[13px]">
                <thead>
                    <tr class="border-b border-divider-soft text-left font-mono text-[11px] uppercase tracking-[0.08em] text-faint-3">
                        <th class="px-6 py-3 font-medium">Date</th>
                        <th class="px-6 py-3 font-medium">Developer</th>
                        <th class="px-6 py-3 font-medium">Issue</th>
                        <th class="px-6 py-3 font-medium">Note</th>
                        <th class="px-6 py-3 text-right font-medium">Time</th>
                    </tr>
                </thead>
                <tbody>
                    <tr v-for="row in rows" :key="row.id" class="border-b border-divider-soft align-top last:border-0">
                        <td class="whitespace-nowrap px-6 py-3 text-faint-2">{{ row.date }}</td>
                        <td class="whitespace-nowrap px-6 py-3 text-muted">{{ row.developer }}</td>
                        <td class="px-6 py-3">
                            <span class="font-mono text-faint-2">{{ row.issueKey }}</span>
                            <span v-if="row.issueTitle" class="ml-2 text-muted">{{ row.issueTitle }}</span>
                        </td>
                        <td class="max-w-[420px] px-6 py-3 leading-[1.5]">{{ row.note }}</td>
                        <td class="whitespace-nowrap px-6 py-3 text-right tabular-nums text-muted">{{ hours(row.minutes) }}</td>
                    </tr>
                </tbody>
            </table>
        </Card>

        <EmptyState
            v-else
            title="No notes found"
            text="No synced worklog notes match this search. Broaden the filters, or sync to pull in fresh worklogs."
        />
    </div>
</template>

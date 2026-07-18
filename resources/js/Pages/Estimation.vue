<script setup>
import { computed, onMounted } from 'vue';
import { router } from '@inertiajs/vue3';
import AppHeader from '../Components/AppHeader.vue';
import Card from '../Components/Card.vue';
import EmptyState from '../Components/EmptyState.vue';
import { usePageCursor } from '../Composables/usePageCursor';
import { useAction } from '../Composables/useAction';

const props = defineProps({
    bias: { type: Object, default: () => ({}) },
    issues: { type: Array, default: () => [] },
    projects: { type: Array, default: () => [] },
    projectIds: { type: Array, default: () => [] },
});

const STORAGE_KEY = 'estimation.projects';

function apply(ids) {
    // Hyphen-joined single param so the URL reads ?projects=3-14. A comma would
    // survive here but Inertia's serializer percent-encodes it (%2C); a hyphen
    // is left untouched.
    const query = ids.length ? { projects: ids.join('-') } : {};
    router.get('/estimation', query, { preserveState: true, preserveScroll: true });
}

// Toggle a project in/out of the filter; empty selection = all projects.
function toggleProject(id) {
    const next = props.projectIds.includes(id)
        ? props.projectIds.filter((p) => p !== id)
        : [...props.projectIds, id];
    localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
    apply(next);
}

// Restore the saved filter on a fresh visit (nav link → no URL params). A hard
// refresh keeps the params on its own, so only reapply when the URL carries none.
onMounted(() => {
    if (props.projectIds.length) return;
    let saved = [];
    try {
        saved = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]');
    } catch {
        saved = [];
    }
    if (Array.isArray(saved) && saved.length) apply(saved);
});

// +% = logged more than estimated (you underestimate); −% = you overestimate.
function biasLabel(pct) {
    if (pct === null) return '—';
    if (pct === 0) return 'on the nose';
    return `${pct > 0 ? '+' : ''}${pct}%`;
}

function biasClass(pct) {
    if (pct === null || pct === 0) return 'text-muted';
    return pct > 0 ? 'text-rose-500' : 'text-emerald-500';
}

// j/k cursor over the bias card then each issue row (#43). String keys: the card
// is 'bias', rows are keyed by issue key.
const focusTargets = computed(() => (props.issues.length ? ['bias', ...props.issues.map((i) => 'iss-' + i.key)] : []));
const cursor = usePageCursor(() => focusTargets.value);

// View-switcher keyset (#45, table #35): `c` clears the project filter (empty =
// all). Clears the saved selection too, so it stays cleared on the next visit.
// Listed in the `?`-palette under `estimation`.
useAction({ id: 'estimation:clear', label: 'Clear project filter', keys: 'c', scope: 'estimation', run: () => {
    localStorage.removeItem(STORAGE_KEY);
    apply([]);
} });
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-11">
        <AppHeader />

        <div class="mb-8">
            <h1 class="mb-3 text-[34px] font-bold leading-[1.05] tracking-[-0.03em]">Estimation</h1>
            <p class="max-w-[62ch] text-[15px] leading-[1.55] text-muted">
                Your own finished issues: the hours you estimated against the hours you actually logged.
                The rolling bias sums the last {{ bias.sampleSize }} estimated issues — a positive figure
                means you tend to underestimate.
            </p>
        </div>

        <template v-if="issues.length">
            <div class="mb-6 flex items-end justify-between gap-6">
                <Card accent pad="22px 28px" data-row="bias" class="min-w-[280px] scroll-my-12" :class="cursor.isActive('bias') && 'ring-2 ring-accent'">
                    <div class="mb-1 font-mono text-[11px] uppercase tracking-[0.16em] text-faint">
                        Rolling estimation bias
                    </div>
                    <div class="flex items-baseline gap-3">
                        <span class="text-[40px] font-bold leading-none tracking-[-0.02em]" :class="biasClass(bias.pct)">
                            {{ biasLabel(bias.pct) }}
                        </span>
                        <span class="text-[13px] text-faint-2">
                            {{ bias.actualHours }}h logged vs {{ bias.estimateHours }}h estimated
                        </span>
                    </div>
                </Card>

                <div v-if="projects.length > 1" class="flex max-w-[560px] flex-col items-end gap-1.5">
                    <span class="font-mono text-[11px] uppercase tracking-[0.08em] text-faint-3">
                        Projects{{ projectIds.length ? ` · ${projectIds.length} selected` : ' · all' }}
                    </span>
                    <div class="flex flex-wrap justify-end gap-1.5">
                        <button
                            v-for="p in projects"
                            :key="p.id"
                            type="button"
                            class="rounded-full border px-3 py-1 text-[12px] transition"
                            :class="
                                projectIds.includes(p.id)
                                    ? 'border-accent bg-accent/10 text-accent'
                                    : 'border-card-border bg-surface text-faint hover:text-muted'
                            "
                            @click="toggleProject(p.id)"
                        >
                            {{ p.name ?? `#${p.id}` }}
                        </button>
                    </div>
                </div>
            </div>

            <Card pad="8px 0">
                <table class="w-full border-collapse text-[13px]">
                    <thead>
                        <tr class="border-b border-divider-soft text-left font-mono text-[11px] uppercase tracking-[0.08em] text-faint-3">
                            <th class="px-6 py-3 font-medium">Issue</th>
                            <th class="px-6 py-3 font-medium">Project</th>
                            <th class="px-6 py-3 font-medium">Last worked</th>
                            <th class="px-6 py-3 text-right font-medium">Estimate</th>
                            <th class="px-6 py-3 text-right font-medium">Actual</th>
                            <th class="px-6 py-3 text-right font-medium">Bias</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr
                            v-for="issue in issues"
                            :key="issue.key"
                            :data-row="'iss-' + issue.key"
                            class="border-b border-divider-soft last:border-0"
                            :class="cursor.isActive('iss-' + issue.key) && 'bg-accent-wash'"
                        >
                            <td class="px-6 py-3">
                                <a :href="issue.kendo_url" target="_blank" class="text-ink hover:text-accent">
                                    <span class="font-mono text-faint-2">{{ issue.key }}</span>
                                    <span class="ml-2">{{ issue.title }}</span>
                                </a>
                            </td>
                            <td class="px-6 py-3 text-muted">{{ issue.project ?? '—' }}</td>
                            <td class="px-6 py-3 text-faint-2">{{ issue.lastWorked ?? '—' }}</td>
                            <td class="px-6 py-3 text-right tabular-nums text-muted">
                                {{ issue.estimateHours !== null ? issue.estimateHours + 'h' : '—' }}
                            </td>
                            <td class="px-6 py-3 text-right tabular-nums">{{ issue.actualHours }}h</td>
                            <td class="px-6 py-3 text-right tabular-nums font-medium" :class="biasClass(issue.biasPct)">
                                {{ biasLabel(issue.biasPct) }}
                            </td>
                        </tr>
                    </tbody>
                </table>
            </Card>
        </template>

        <EmptyState
            v-else
            title="No finished issues yet"
            text="Once your assigned issues reach the Done lane in Kendo, they'll show up here with estimate vs actual. Sync to refresh."
        />
    </div>
</template>

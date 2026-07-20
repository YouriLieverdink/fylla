<script setup>
import { computed, nextTick, ref } from 'vue';
import { router } from '@inertiajs/vue3';
import AppButton from '../Components/AppButton.vue';
import AppHeader from '../Components/AppHeader.vue';
import Card from '../Components/Card.vue';
import DeliveryProjectionChart from '../Components/DeliveryProjectionChart.vue';
import EmptyState from '../Components/EmptyState.vue';
import ProjectRow from '../Components/ProjectRow.vue';
import SegmentedControl from '../Components/SegmentedControl.vue';
import { usePageCursor } from '../Composables/usePageCursor';
import { useModalGuard } from '../Composables/useModalGuard';
import { useAction } from '../Composables/useAction';

const props = defineProps({
    clients: { type: Array, default: () => [] },
    projects: { type: Array, default: () => [] },
});

// By-client cards (#62) vs By-project flat list (#64). By project is the sole
// billable editor for unassigned/yours-only projects, which the cards omit.
const view = ref('By client');

// j/k cursor over the active view's rows (#43): projection cards under By
// client, flat project rows under By project.
const cursor = usePageCursor(() =>
    view.value === 'By client'
        ? props.clients.map((c) => 'd-' + c.id)
        : props.projects.map((p) => 'pr-' + p.id),
);

// View-switcher keyset (#45), migrated from the old Clients page (#64).
useAction({ id: 'delivery:by-client', label: 'By client', keys: 'c', scope: 'delivery', run: () => (view.value = 'By client') });
useAction({ id: 'delivery:by-project', label: 'By project', keys: 'p', scope: 'delivery', run: () => (view.value = 'By project') });

// A client's assigned projects — drives its billable pills.
const assigned = (clientId) => props.projects.filter((p) => p.client_id === clientId);

// The three config modals (#63): new-client, add-project (per card), delete
// (per card). At most one is open at a time — the guard's single-layer
// invariant. Escape closes whichever is open (its own handler on each scrim);
// every keybinding beneath the scrim is suppressed while open (#43).
const creating = ref(false);
const addingTo = ref(null);
const deleting = ref(null);
useModalGuard(() => creating.value || addingTo.value !== null || deleting.value !== null);

const newName = ref('');
const newTarget = ref('');
const nameInput = ref(null);
const search = ref('');
const searchInput = ref(null);

function focusSearch() {
    nextTick(() => searchInput.value?.focus());
}

// `autofocus` doesn't re-fire on a v-if mount, so focus explicitly on open.
function openCreate() {
    creating.value = true;
    nextTick(() => nameInput.value?.focus());
}

// Unassigned projects (ADR-0011 pseudo-clients) matching the add-project search.
const unassigned = computed(() => props.projects.filter((p) => !p.client_id));
const addable = computed(() => {
    const q = search.value.trim().toLowerCase();
    return unassigned.value.filter(
        (p) => !q || p.name.toLowerCase().includes(q) || (p.code || '').toLowerCase().includes(q),
    );
});

// Footer edits hit the existing write routes (no new endpoints, #62). Card pills
// pass the flipped state; flat rows (#64) pass the checkbox's emitted state.
function setBillable(project, billable) {
    router.patch('/projects/' + project.id, { billable }, { preserveScroll: true });
}

function setTarget(card, value) {
    router.patch(
        '/clients/' + card.id,
        { monthly_target_hours: value === '' ? null : Number(value) },
        { preserveScroll: true },
    );
}

function createClient() {
    if (!newName.value.trim()) return;
    router.post(
        '/clients',
        { name: newName.value.trim(), monthly_target_hours: newTarget.value === '' ? null : Number(newTarget.value) },
        {
            preserveScroll: true,
            onSuccess: () => {
                newName.value = '';
                newTarget.value = '';
                creating.value = false;
            },
        },
    );
}

function openAdd(card) {
    addingTo.value = card;
    search.value = '';
    focusSearch();
}

// Assign, but keep the dialog open (focus back on search) to add several in a row.
function pickProject(project) {
    router.patch('/projects/' + project.id, { client_id: addingTo.value.id }, { preserveScroll: true });
    focusSearch();
}

function deleteClient() {
    router.delete('/clients/' + deleting.value.id, {
        preserveScroll: true,
        onSuccess: () => (deleting.value = null),
    });
}
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-11">
        <AppHeader />

        <div class="mb-8 flex items-center gap-6">
            <div class="flex-[7]">
                <h1 class="mb-3 text-[34px] font-bold leading-[1.05] tracking-[-0.03em]">Delivery</h1>
                <p class="max-w-[62ch] text-[15px] leading-[1.55] text-muted">
                    Team-aggregate hours delivered this month per client — every developer's worklogs plus
                    your own, billable and non-billable, against each client's monthly target. Set targets,
                    assign projects, and flag billable inline; clients without a target show delivered hours alone.
                </p>
            </div>
            <div class="flex flex-[2] justify-end">
                <AppButton size="sm" @click="openCreate">+ New client</AppButton>
            </div>
        </div>

        <div class="mb-5">
            <SegmentedControl v-model="view" :options="['By client', 'By project']" />
        </div>

        <template v-if="view === 'By client'">
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
                                @click="setBillable(p, !p.billable)"
                            >
                                <span class="h-1.5 w-1.5 rounded-full" :class="p.billable ? 'bg-accent' : 'bg-faint-3'"></span>
                                {{ p.name }}
                            </button>
                        </div>

                        <div class="flex items-center gap-3">
                            <button type="button" class="cursor-pointer font-mono text-[11px] font-semibold uppercase tracking-[0.1em] text-faint-3 transition hover:text-ink" @click="openAdd(c)">
                                + project
                            </button>
                            <button type="button" class="cursor-pointer font-mono text-[11px] font-semibold uppercase tracking-[0.1em] text-faint-3 transition hover:text-behind" @click="deleting = c">
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
            text="Add one with + New client above, then assign it Kendo projects — its team's monthly hours will show up here."
        />
        </template>

        <!-- By project: flat list of all projects (assigned + unassigned), name +
             billable. Sole billable editor for unassigned/yours-only work (#64). -->
        <Card v-else-if="projects.length" radius="22px" pad="10px 10px 12px">
            <div class="grid grid-cols-[1fr_auto] gap-3 px-2 pb-3.5 pt-3 font-mono text-[10px] font-semibold uppercase tracking-[0.1em] text-faint-3">
                <span>Project</span>
                <span>Billable</span>
            </div>
            <div class="flex flex-col">
                <ProjectRow
                    v-for="project in projects"
                    :key="project.id"
                    :project="project"
                    :data-row="'pr-' + project.id"
                    class="scroll-my-12"
                    :class="cursor.isActive('pr-' + project.id) && 'ring-2 ring-accent'"
                    @toggle-billable="setBillable(project, $event)"
                />
            </div>
        </Card>
        <EmptyState v-else title="No projects synced yet" text="Run Sync now to pull your Kendo projects in." />

        <!-- New-client modal (#63) -->
        <div
            v-if="creating"
            class="fixed inset-0 z-50 flex items-start justify-center bg-black/30 px-4 pt-[15vh]"
            @click.self="creating = false"
            @keydown.esc.window="creating = false"
        >
            <Card radius="18px" pad="16px 18px" class="w-full max-w-[440px]">
                <h2 class="mb-3 text-[15px] font-semibold">New client</h2>
                <form class="flex flex-col gap-3" @submit.prevent="createClient">
                    <input
                        ref="nameInput"
                        v-model="newName"
                        type="text"
                        placeholder="Client name"
                        class="w-full rounded-[11px] border border-[#e0dbd0] bg-surface px-3.5 py-2.5 text-[14px] outline-none focus:border-accent"
                    />
                    <input
                        v-model="newTarget"
                        type="number"
                        min="0"
                        placeholder="Target h/mo (optional)"
                        class="w-full rounded-[11px] border border-[#e0dbd0] bg-surface px-3.5 py-2.5 text-[14px] outline-none focus:border-accent"
                    />
                    <div class="flex justify-end gap-2">
                        <AppButton type="button" variant="ghost" size="sm" @click="creating = false">Cancel</AppButton>
                        <AppButton type="submit" size="sm">Add client</AppButton>
                    </div>
                </form>
            </Card>
        </div>

        <!-- Add-project search modal, mirrors the old Clients dialog (#63) -->
        <div
            v-if="addingTo"
            class="fixed inset-0 z-50 flex items-start justify-center bg-black/30 px-4 pt-[15vh]"
            @click.self="addingTo = null"
            @keydown.esc.window="addingTo = null"
        >
            <Card radius="18px" pad="16px 18px" class="w-full max-w-[440px]">
                <div class="mb-3 flex items-center justify-between">
                    <h2 class="text-[15px] font-semibold">Add project to {{ addingTo.name }}</h2>
                    <button class="cursor-pointer font-mono text-[11px] uppercase tracking-[0.1em] text-faint-3 hover:text-ink" @click="addingTo = null">
                        Done
                    </button>
                </div>
                <input
                    ref="searchInput"
                    v-model="search"
                    type="text"
                    placeholder="Search projects…"
                    class="mb-2 w-full rounded-[11px] border border-[#e0dbd0] bg-surface px-3.5 py-2.5 text-[14px] outline-none focus:border-accent"
                />
                <div class="max-h-[46vh] overflow-y-auto">
                    <button
                        v-for="p in addable"
                        :key="p.id"
                        class="flex w-full cursor-pointer items-center gap-2 rounded-[10px] px-3 py-2.5 text-left transition hover:bg-canvas"
                        @click="pickProject(p)"
                    >
                        <span class="min-w-0 flex-1 truncate text-[14px]">{{ p.name }}</span>
                        <span v-if="p.code" class="font-mono text-[11px] text-faint-3">{{ p.code }}</span>
                    </button>
                    <p v-if="!addable.length" class="px-3 py-3 text-[13px] text-faint">No unassigned projects match.</p>
                </div>
            </Card>
        </div>

        <!-- Delete-client confirm modal (#63) -->
        <div
            v-if="deleting"
            class="fixed inset-0 z-50 flex items-start justify-center bg-black/30 px-4 pt-[15vh]"
            @click.self="deleting = null"
            @keydown.esc.window="deleting = null"
        >
            <Card radius="18px" pad="16px 18px" class="w-full max-w-[440px]">
                <h2 class="mb-2 text-[15px] font-semibold">Delete {{ deleting.name }}?</h2>
                <p class="mb-4 text-[13px] leading-[1.55] text-muted">
                    Its projects return to <strong class="font-semibold text-ink-soft">unassigned</strong>
                    (yours-only, ADR-0011). Worklog history is <strong class="font-semibold text-ink-soft">kept</strong>.
                    This <strong class="font-semibold text-ink-soft">can't be undone</strong>.
                </p>
                <div class="flex justify-end gap-2">
                    <AppButton type="button" variant="ghost" size="sm" @click="deleting = null">Cancel</AppButton>
                    <AppButton type="button" size="sm" @click="deleteClient">Delete client</AppButton>
                </div>
            </Card>
        </div>
    </div>
</template>

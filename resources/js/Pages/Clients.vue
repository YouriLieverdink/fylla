<script setup>
import { ref, computed, nextTick } from 'vue';
import { router } from '@inertiajs/vue3';
import Card from '../Components/Card.vue';
import AppHeader from '../Components/AppHeader.vue';
import AppButton from '../Components/AppButton.vue';
import SegmentedControl from '../Components/SegmentedControl.vue';
import ProjectRow from '../Components/ProjectRow.vue';
import { usePageCursor } from '../Composables/usePageCursor';

const props = defineProps({
    projects: { type: Array, default: () => [] },
    clients: { type: Array, default: () => [] },
});

const view = ref('By client');

const newName = ref('');
const newTarget = ref('');

// The client whose "add project" dialog is open, plus its search query.
const addingTo = ref(null);
const search = ref('');
const searchInput = ref(null);

function focusSearch() {
    nextTick(() => searchInput.value?.focus());
}

// Real clients with their assigned projects; unassigned projects fall through
// to their own-name pseudo-clients (ADR-0011).
const groups = computed(() =>
    props.clients.map((client) => ({
        client,
        projects: props.projects.filter((p) => p.client_id === client.id),
    })),
);
const unassigned = computed(() => props.projects.filter((p) => !p.client_id));

// Unassigned projects matching the dialog search box.
const addable = computed(() => {
    const q = search.value.trim().toLowerCase();
    return unassigned.value.filter(
        (p) => !q || p.name.toLowerCase().includes(q) || (p.code || '').toLowerCase().includes(q),
    );
});

function toggleBillable(project, billable) {
    router.patch('/projects/' + project.id, { billable }, { preserveScroll: true });
}

function assign(project, clientId) {
    router.patch('/projects/' + project.id, { client_id: clientId }, { preserveScroll: true });
}

function openAdd(client) {
    addingTo.value = client;
    search.value = '';
    focusSearch();
}

// Assign, but keep the dialog open (focus back on search) to add several in a row.
function pickProject(project) {
    assign(project, addingTo.value.id);
    focusSearch();
}

function createClient() {
    if (!newName.value.trim()) return;
    router.post(
        '/clients',
        { name: newName.value.trim(), monthly_target_hours: newTarget.value === '' ? null : Number(newTarget.value) },
        { preserveScroll: true, onSuccess: () => { newName.value = ''; newTarget.value = ''; } },
    );
}

function renameClient(client, name) {
    const trimmed = name.trim();
    if (trimmed && trimmed !== client.name) {
        router.patch('/clients/' + client.id, { name: trimmed }, { preserveScroll: true });
    }
}

function setTarget(client, value) {
    router.patch(
        '/clients/' + client.id,
        { monthly_target_hours: value === '' ? null : Number(value) },
        { preserveScroll: true },
    );
}

function deleteClient(client) {
    router.delete('/clients/' + client.id, { preserveScroll: true });
}

// j/k cursor over the active view's cards/rows (#43): client cards under "By
// client", flat project rows under "By project".
const focusTargets = computed(() =>
    view.value === 'By client'
        ? groups.value.map((g) => 'cl-' + g.client.id)
        : props.projects.map((p) => 'pr-' + p.id),
);
const cursor = usePageCursor(() => focusTargets.value);
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-11">
        <AppHeader />

        <div class="mb-8">
            <h1 class="mb-3 text-[34px] font-bold leading-[1.05] tracking-[-0.03em]">Clients</h1>
            <p class="max-w-[62ch] text-[15px] leading-[1.55] text-muted">
                Group Kendo projects under a client to manage the whole team's hours against a monthly
                target. Assigning a project pulls in teammates' worklogs; unassigned projects stay
                <strong class="font-semibold text-ink-soft">yours-only</strong>. Flag each project
                <strong class="font-semibold text-ink-soft">billable</strong> to count its hours toward utilization.
            </p>
        </div>

        <div class="mb-5">
            <SegmentedControl v-model="view" :options="['By client', 'By project']" />
        </div>

        <!-- New client (client tab only) -->
        <Card v-if="view === 'By client'" radius="20px" pad="16px 18px" class="mb-4">
            <form class="flex flex-wrap items-center gap-3" @submit.prevent="createClient">
                <input
                    v-model="newName"
                    type="text"
                    placeholder="New client name"
                    class="min-w-0 flex-1 rounded-[11px] border border-[#e0dbd0] bg-surface px-3.5 py-2.5 text-[14px] outline-none focus:border-accent"
                />
                <input
                    v-model="newTarget"
                    type="number"
                    min="0"
                    placeholder="Target h/mo"
                    class="w-[130px] rounded-[11px] border border-[#e0dbd0] bg-surface px-3.5 py-2.5 text-[14px] outline-none focus:border-accent"
                />
                <AppButton type="submit" size="sm">Add client</AppButton>
            </form>
        </Card>

        <!-- By client: half-width client cards -->
        <template v-if="view === 'By client'">
            <div v-if="clients.length" class="grid grid-cols-1 gap-4 md:grid-cols-2">
                <Card v-for="{ client, projects: assigned } in groups" :key="client.id" :data-row="'cl-' + client.id" radius="22px" pad="18px 20px 16px" class="scroll-my-12" :class="cursor.isActive('cl-' + client.id) && 'ring-2 ring-accent'">
                    <div class="mb-3 flex items-center gap-3">
                        <input
                            :value="client.name"
                            class="min-w-0 flex-1 rounded-[10px] border border-transparent bg-transparent px-2 py-1 text-[17px] font-semibold tracking-[-0.01em] outline-none hover:border-divider-soft focus:border-accent focus:bg-surface"
                            @change="renameClient(client, $event.target.value)"
                        />
                        <label class="flex items-center gap-1.5 font-mono text-[11px] uppercase tracking-[0.1em] text-faint-3">
                            <input
                                type="number"
                                min="0"
                                :value="client.monthly_target_hours ?? ''"
                                placeholder="—"
                                class="w-[64px] rounded-[9px] border border-[#e0dbd0] bg-surface px-2 py-1 text-right font-sans text-[13px] tabular-nums outline-none focus:border-accent"
                                @change="setTarget(client, $event.target.value)"
                            />
                            h/mo
                        </label>
                        <button
                            class="cursor-pointer rounded-[9px] px-2 py-1 font-mono text-[11px] font-semibold uppercase tracking-[0.1em] text-faint-3 transition hover:bg-behind-tint hover:text-behind"
                            @click="deleteClient(client)"
                        >
                            Delete
                        </button>
                    </div>

                    <div v-if="assigned.length" class="flex flex-col">
                        <ProjectRow
                            v-for="project in assigned"
                            :key="project.id"
                            :project="project"
                            @toggle-billable="toggleBillable(project, $event)"
                        >
                            <template #action>
                                <button
                                    class="cursor-pointer font-mono text-[11px] font-semibold uppercase tracking-[0.1em] text-faint-3 transition hover:text-behind"
                                    @click="assign(project, null)"
                                >
                                    Remove
                                </button>
                            </template>
                        </ProjectRow>
                    </div>
                    <p v-else class="border-t border-divider-soft px-2 py-3 text-[13px] text-faint">No projects assigned.</p>

                    <button
                        v-if="unassigned.length"
                        class="mt-3 w-full cursor-pointer rounded-[11px] border border-dashed border-[#e0dbd0] py-2 text-[13px] font-medium text-muted transition hover:bg-canvas"
                        @click="openAdd(client)"
                    >
                        + Add project
                    </button>
                </Card>
            </div>

            <p v-else class="text-[14px] text-faint">
                No clients yet. Add one above, then use its
                <strong class="font-semibold text-ink-soft">Add project</strong> button to pull projects in.
            </p>
        </template>

        <!-- By project: flat list, name + billable -->
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
                    @toggle-billable="toggleBillable(project, $event)"
                />
            </div>
        </Card>

        <div v-if="!projects.length" class="text-[14px] text-faint">No projects synced yet.</div>

        <!-- Add-project dialog -->
        <div
            v-if="addingTo"
            class="fixed inset-0 z-50 flex items-start justify-center bg-black/30 px-4 pt-[15vh]"
            @click.self="addingTo = null"
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
    </div>
</template>

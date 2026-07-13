<script setup>
import { router } from '@inertiajs/vue3';
import Card from '../Components/Card.vue';
import AppHeader from '../Components/AppHeader.vue';

defineProps({
    projects: { type: Array, default: () => [] },
});

// Flip billable on toggle; the worklog metric re-derives on next read (ADR-0007).
function toggle(project, billable) {
    router.patch('/projects/' + project.id, { billable }, { preserveScroll: true });
}
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-11">
        <AppHeader />

        <Card v-if="projects.length" radius="24px" pad="10px 10px 12px">
            <div
                class="grid grid-cols-[1fr_auto] gap-3 px-5 pb-3.5 pt-4 font-mono text-[10px] font-semibold uppercase tracking-[0.1em] text-faint-3"
            >
                <span>Project</span>
                <span>Billable</span>
            </div>

            <div class="flex flex-col">
                <label
                    v-for="project in projects"
                    :key="project.id"
                    class="grid cursor-pointer grid-cols-[1fr_auto] items-center gap-3 rounded-[14px] border-t border-divider-soft px-5 py-3.5 transition hover:bg-surface-soft"
                >
                    <div class="min-w-0">
                        <div class="truncate text-[14px] font-medium">{{ project.name }}</div>
                        <div v-if="project.code" class="mt-[3px] font-mono text-[11px] text-faint-3">{{ project.code }}</div>
                    </div>
                    <input
                        type="checkbox"
                        class="h-4 w-4 accent-accent"
                        :checked="project.billable"
                        @change="toggle(project, $event.target.checked)"
                    />
                </label>
            </div>
        </Card>

        <div v-else class="text-[14px] text-faint">No projects synced yet.</div>
    </div>
</template>

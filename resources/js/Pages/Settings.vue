<script setup>
import { useForm } from '@inertiajs/vue3';
import AppHeader from '../Components/AppHeader.vue';
import AppButton from '../Components/AppButton.vue';
import Card from '../Components/Card.vue';

const props = defineProps({
    values: { type: Object, required: true },
});

const weekdays = [
    { value: 1, label: 'Monday' },
    { value: 2, label: 'Tuesday' },
    { value: 3, label: 'Wednesday' },
    { value: 4, label: 'Thursday' },
    { value: 5, label: 'Friday' },
    { value: 6, label: 'Saturday' },
    { value: 7, label: 'Sunday' },
];

// List fields edit as one-entry-per-line textareas; split on submit.
const form = useForm({
    ...props.values,
    github_pr_queries: (props.values.github_pr_queries ?? []).join('\n'),
    github_pr_exclude_repos: (props.values.github_pr_exclude_repos ?? []).join('\n'),
});

const lines = (s) => String(s).split('\n').map((l) => l.trim()).filter(Boolean);

function submit() {
    form
        .transform((d) => ({
            ...d,
            github_pr_queries: lines(d.github_pr_queries),
            github_pr_exclude_repos: lines(d.github_pr_exclude_repos),
        }))
        .put('/settings', { preserveScroll: true });
}

const field = 'w-full rounded-[11px] border border-[#e0dbd0] bg-surface px-3.5 py-2.5 text-[14px] outline-none focus:border-accent';
</script>

<template>
    <div class="mx-auto max-w-[1180px] px-11 pb-[120px] pt-11">
        <AppHeader />

        <div class="mb-8">
            <h1 class="mb-3 text-[34px] font-bold leading-[1.05] tracking-[-0.03em]">Settings</h1>
            <p class="max-w-[62ch] text-[15px] leading-[1.55] text-muted">
                The tuning knobs behind Fylla. Each value defaults to what's in
                <code class="text-[13px]">config/fylla.php</code>; saving here overrides that default.
            </p>
        </div>

        <form class="flex flex-col gap-4" @submit.prevent="submit">
            <!-- Utilization -->
            <Card radius="20px" pad="20px 24px">
                <h2 class="mb-4 font-mono text-[11px] uppercase tracking-[0.16em] text-faint">Utilization</h2>
                <div class="grid grid-cols-2 gap-4">
                    <label class="flex flex-col gap-1.5">
                        <span class="text-[13px] text-muted">Target (%)</span>
                        <input v-model.number="form.utilization_target" type="number" min="0" max="100" :class="field" />
                        <span v-if="form.errors.utilization_target" class="text-[12px] text-rose-500">{{ form.errors.utilization_target }}</span>
                    </label>
                    <label class="flex flex-col gap-1.5">
                        <span class="text-[13px] text-muted">Soft floor (%)</span>
                        <input v-model.number="form.utilization_soft_floor" type="number" min="0" max="100" :class="field" />
                        <span v-if="form.errors.utilization_soft_floor" class="text-[12px] text-rose-500">{{ form.errors.utilization_soft_floor }}</span>
                    </label>
                    <label class="flex flex-col gap-1.5">
                        <span class="text-[13px] text-muted">Contracted hours / week</span>
                        <input v-model.number="form.contracted_hours_per_week" type="number" min="1" :class="field" />
                        <span v-if="form.errors.contracted_hours_per_week" class="text-[12px] text-rose-500">{{ form.errors.contracted_hours_per_week }}</span>
                    </label>
                    <label class="flex flex-col gap-1.5">
                        <span class="text-[13px] text-muted">Day off</span>
                        <select v-model.number="form.contracted_off_weekday" :class="field">
                            <option v-for="d in weekdays" :key="d.value" :value="d.value">{{ d.label }}</option>
                        </select>
                        <span v-if="form.errors.contracted_off_weekday" class="text-[12px] text-rose-500">{{ form.errors.contracted_off_weekday }}</span>
                    </label>
                    <label class="flex flex-col gap-1.5">
                        <span class="text-[13px] text-muted">Trend window (weeks)</span>
                        <input v-model.number="form.utilization_window_weeks" type="number" min="1" :class="field" />
                        <span v-if="form.errors.utilization_window_weeks" class="text-[12px] text-rose-500">{{ form.errors.utilization_window_weeks }}</span>
                    </label>
                </div>
            </Card>

            <!-- Sync -->
            <Card radius="20px" pad="20px 24px">
                <h2 class="mb-4 font-mono text-[11px] uppercase tracking-[0.16em] text-faint">Sync</h2>
                <div class="grid grid-cols-2 gap-4">
                    <label class="flex flex-col gap-1.5">
                        <span class="text-[13px] text-muted">Worklog sync window (days)</span>
                        <input v-model.number="form.worklog_sync_days" type="number" min="1" :class="field" />
                        <span v-if="form.errors.worklog_sync_days" class="text-[12px] text-rose-500">{{ form.errors.worklog_sync_days }}</span>
                    </label>
                    <label class="flex flex-col gap-1.5">
                        <span class="text-[13px] text-muted">Kendo user id</span>
                        <input v-model="form.kendo_user_id" type="text" :class="field" />
                        <span class="text-[12px] text-faint-3">Wrong value silently empties the worklog.</span>
                        <span v-if="form.errors.kendo_user_id" class="text-[12px] text-rose-500">{{ form.errors.kendo_user_id }}</span>
                    </label>
                </div>
            </Card>

            <!-- GitHub PRs -->
            <Card radius="20px" pad="20px 24px">
                <h2 class="mb-4 font-mono text-[11px] uppercase tracking-[0.16em] text-faint">GitHub PRs</h2>
                <div class="flex flex-col gap-4">
                    <label class="flex flex-col gap-1.5">
                        <span class="text-[13px] text-muted">PR queries — one per line (<code class="text-[12px]">is:pr is:open</code> is prepended)</span>
                        <textarea v-model="form.github_pr_queries" rows="3" :class="field" class="font-mono text-[13px]"></textarea>
                        <span v-if="form.errors.github_pr_queries" class="text-[12px] text-rose-500">{{ form.errors.github_pr_queries }}</span>
                    </label>
                    <label class="flex flex-col gap-1.5">
                        <span class="text-[13px] text-muted">Excluded repos (owner/name) — one per line</span>
                        <textarea v-model="form.github_pr_exclude_repos" rows="2" :class="field" class="font-mono text-[13px]"></textarea>
                        <span v-if="form.errors.github_pr_exclude_repos" class="text-[12px] text-rose-500">{{ form.errors.github_pr_exclude_repos }}</span>
                    </label>
                </div>
            </Card>

            <!-- Display -->
            <Card radius="20px" pad="20px 24px">
                <h2 class="mb-4 font-mono text-[11px] uppercase tracking-[0.16em] text-faint">Display</h2>
                <label class="flex max-w-[320px] flex-col gap-1.5">
                    <span class="text-[13px] text-muted">Timezone</span>
                    <input v-model="form.display_timezone" type="text" :class="field" />
                    <span v-if="form.errors.display_timezone" class="text-[12px] text-rose-500">{{ form.errors.display_timezone }}</span>
                </label>
            </Card>

            <div class="flex items-center gap-3">
                <AppButton type="submit" :disabled="form.processing">Save settings</AppButton>
                <span v-if="form.recentlySuccessful" class="text-[13px] text-emerald-500">Saved.</span>
            </div>
        </form>
    </div>
</template>

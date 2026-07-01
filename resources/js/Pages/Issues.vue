<script setup>
import { router } from '@inertiajs/vue3';

defineProps({
    issues: { type: Array, default: () => [] },
    lastSyncedAt: { type: String, default: null },
});

function syncNow() {
    router.post('/sync', {}, { preserveScroll: true });
}

function fmt(ts) {
    return ts ? new Date(ts).toLocaleString() : '—';
}
</script>

<template>
    <div class="mx-auto max-w-4xl p-8">
        <div class="mb-6 flex items-center justify-between">
            <h1 class="text-2xl font-semibold">My Kendo issues</h1>
            <button
                class="rounded bg-black px-4 py-2 text-sm text-white hover:opacity-80"
                @click="syncNow"
            >
                Sync now
            </button>
        </div>

        <p class="mb-4 text-sm text-gray-500">Last synced: {{ fmt(lastSyncedAt) }}</p>

        <table class="w-full border-collapse text-sm">
            <thead>
                <tr class="border-b text-left text-gray-500">
                    <th class="py-2 pr-4">Key</th>
                    <th class="py-2 pr-4">Title</th>
                    <th class="py-2 pr-4">Priority</th>
                    <th class="py-2 pr-4">Type</th>
                    <th class="py-2">Updated</th>
                </tr>
            </thead>
            <tbody>
                <tr v-for="issue in issues" :key="issue.key" class="border-b">
                    <td class="py-2 pr-4 font-mono">{{ issue.key }}</td>
                    <td class="py-2 pr-4">{{ issue.title }}</td>
                    <td class="py-2 pr-4">{{ issue.priority }}</td>
                    <td class="py-2 pr-4">{{ issue.type }}</td>
                    <td class="py-2 text-gray-500">{{ fmt(issue.updated_at) }}</td>
                </tr>
                <tr v-if="issues.length === 0">
                    <td colspan="5" class="py-6 text-center text-gray-400">
                        No issues yet — hit “Sync now”.
                    </td>
                </tr>
            </tbody>
        </table>
    </div>
</template>

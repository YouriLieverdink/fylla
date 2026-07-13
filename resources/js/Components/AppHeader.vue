<script setup>
import { computed, ref } from 'vue';
import { router, usePage, usePoll } from '@inertiajs/vue3';
import Nav from './Nav.vue';
import SyncStatus from './SyncStatus.vue';

// lastSyncedAt + syncError are globally shared (HandleInertiaRequests), so the
// header is identical on every page with no per-page props.
const page = usePage();
const lastSyncedAt = computed(() => page.props.lastSyncedAt);
const syncError = computed(() => page.props.syncError);
const syncing = ref(false);

// Keep the "last synced" label fresh across the 15-min scheduled sync.
usePoll(60000, { only: ['lastSyncedAt'] });

function syncNow() {
    router.post('/sync', {}, {
        preserveScroll: true,
        onStart: () => (syncing.value = true),
        onFinish: () => (syncing.value = false),
    });
}

function fmt(ts) {
    return ts ? new Date(ts).toLocaleString() : '—';
}
</script>

<template>
    <header class="mb-[34px] flex items-center justify-between gap-6 border-b border-divider-soft pb-[26px]">
        <Nav />
        <SyncStatus
            label="Synced with issue tracker"
            :last-synced="lastSyncedAt ? 'last synced ' + fmt(lastSyncedAt) : 'never synced'"
            :syncing="syncing"
            :error="syncError"
            @sync="syncNow"
        />
    </header>
</template>

<script setup>
import { computed, ref } from 'vue';
import { Link, router, usePage, usePoll } from '@inertiajs/vue3';
import Nav from './Nav.vue';
import SyncStatus from './SyncStatus.vue';

const settingsActive = computed(() => usePage().url.startsWith('/settings'));

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
        <div class="flex items-center gap-5">
            <SyncStatus
                label="Synced with issue tracker"
                :last-synced="lastSyncedAt ? 'last synced ' + fmt(lastSyncedAt) : 'never synced'"
                :syncing="syncing"
                :error="syncError"
                @sync="syncNow"
            />
            <Link
                href="/settings"
                aria-label="Settings"
                class="transition"
                :class="settingsActive ? 'text-ink' : 'text-faint hover:text-muted'"
            >
                <svg class="h-[18px] w-[18px]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <circle cx="12" cy="12" r="3" />
                    <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
                </svg>
            </Link>
        </div>
    </header>
</template>

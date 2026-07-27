<script setup>
import { computed, ref, watch } from 'vue';
import { Link, router, usePage } from '@inertiajs/vue3';
import { useActivityChannel } from '../Composables/useActivityChannel';
import Nav from './Nav.vue';
import SyncStatus from './SyncStatus.vue';

const settingsActive = computed(() => usePage().url.startsWith('/settings'));
const activityActive = computed(() => usePage().url.startsWith('/activity'));

// lastSyncedAt + activityRunning + activityFailures are globally shared
// (HandleInertiaRequests), so the header is identical on every page with no
// per-page props.
const page = usePage();
const lastSyncedAt = computed(() => page.props.lastSyncedAt);
const activityRunning = computed(() => page.props.activityRunning);
const activityFailures = computed(() => page.props.activityFailures);
const posting = ref(false);

const syncing = computed(() => posting.value || activityRunning.value);

// The dispatch is accepted long before a worker picks it up, so a spinner alone
// would just stop with nothing to show for it. "Queued" holds that gap and
// clears the moment a run goes live — and if it never clears, that itself is
// the signal that `queue:work` isn't running.
const queued = ref(false);
watch(activityRunning, (running) => {
    if (running) queued.value = false;
});

function syncNow() {
    router.post('/sync', {}, {
        preserveScroll: true,
        onStart: () => (posting.value = true),
        onSuccess: () => (queued.value = true),
        onFinish: () => (posting.value = false),
    });
}

// The "Sync now" POST returns before any job starts, so the running state only
// ever arrives out-of-band — pushed over the socket now (#92). All three keys:
// with fewer, "Sync now" stops clearing its own timestamp and failure dot.
const { online } = useActivityChannel(['lastSyncedAt', 'activityRunning', 'activityFailures']);

function fmt(ts) {
    return ts ? new Date(ts).toLocaleString() : '—';
}
</script>

<template>
    <header class="mb-[34px] flex items-center justify-between gap-6 border-b border-divider-soft pb-[26px]">
        <Nav />
        <div class="flex items-center gap-5">
            <!-- Socket down: the header's live state is frozen, so it says so. -->
            <span
                v-if="!online"
                class="h-2 w-2 rounded-full bg-faint-2"
                title="Not connected — live activity is paused"
                aria-label="Not connected"
            ></span>
            <SyncStatus
                label="Synced with issue tracker"
                :last-synced="lastSyncedAt ? 'last synced ' + fmt(lastSyncedAt) : 'never synced'"
                :syncing="syncing"
                @sync="syncNow"
            />
            <Link
                href="/activity"
                aria-label="Activity"
                class="relative transition"
                :class="activityRunning ? 'text-accent' : activityActive ? 'text-ink' : 'text-faint hover:text-muted'"
            >
                <!-- Mid-flight: the same spinning refresh arrow the runs use on /activity. -->
                <svg
                    v-if="activityRunning"
                    class="h-[18px] w-[18px] animate-spin"
                    viewBox="0 0 14 14"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.5"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                >
                    <path d="M12 7a5 5 0 1 1-1.46-3.54M12 2v3h-3" />
                </svg>
                <svg v-else class="h-[18px] w-[18px]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M22 12h-4l-3 9L9 3l-3 9H2" />
                </svg>
                <span
                    v-if="activityFailures"
                    class="absolute -right-1 -top-1 h-2 w-2 rounded-full bg-behind ring-2 ring-surface"
                    aria-label="Recent failures"
                ></span>
            </Link>
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

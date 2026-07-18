<script setup>
import { computed, nextTick, ref, watch } from 'vue';
import { registry, useAction } from '../Composables/useAction';
import { useModalGuard } from '../Composables/useModalGuard';

// `?`-triggered cheat-sheet overlay (#41, prototype variant C of #32): an
// on-demand, searchable reference of every live binding, grouped by scope.
// Read-only — not a command palette, no always-on HUD, no mnemonic hints.
//
// The toggle registers through useAction like any other binding, so it rides
// the layout's one guarded tinykeys listener (no typing '?' in an input opens
// it) and lists itself in the sheet. Escape closes — handled locally since it
// only means "close" while the overlay is open.
const open = ref(false);
const query = ref('');
const input = ref(null);

// Modal guard (#43): the cheat-sheet is a blocking modal — while open it
// suppresses every keybinding beneath it (its own toggle included, so `?` and
// Escape close it via the local handlers below, not the registry).
useModalGuard(() => open.value);

useAction({
    id: 'help',
    label: 'Keyboard shortcuts',
    keys: 'Shift+?',
    scope: 'global',
    // preventDefault or the '?' char lands in the search input once we focus it
    // on the same keypress that opened the overlay.
    run: (event) => {
        event.preventDefault();
        open.value = !open.value;
    },
});

// Static Navigation section (#34/#42): the j/k/digit cursor keys register in the
// `navigation` scope but are described here as a fixed pair of rows, not one entry
// per digit. Filterable like the rest.
const NAV_HELP = [
    { label: 'Move cursor / scroll page', keys: ['j', 'k'] },
    { label: 'Jump to row 1–9', keys: ['1', '–', '9'] },
    { label: 'Jump to top / bottom', keys: ['g', 'g', '/', 'G'] },
];
const navRows = computed(() => {
    const q = query.value.trim().toLowerCase();
    if (!q) return NAV_HELP;
    return NAV_HELP.filter((r) => r.label.toLowerCase().includes(q) || r.keys.join(' ').toLowerCase().includes(q));
});

// Group the (optionally filtered) live registry by scope for per-scope headers.
// The `navigation` scope is rendered as the static section above, not here.
const groups = computed(() => {
    const q = query.value.trim().toLowerCase();
    const byScope = new Map();
    for (const a of registry.values()) {
        if (a.scope === 'navigation') continue;
        if (q && !a.label.toLowerCase().includes(q) && !a.keys.toLowerCase().includes(q)) continue;
        if (!byScope.has(a.scope)) byScope.set(a.scope, []);
        byScope.get(a.scope).push(a);
    }
    return [...byScope.entries()].map(([scope, actions]) => ({
        scope,
        actions: actions.sort((x, y) => x.label.localeCompare(y.label)),
    }));
});

function close() {
    open.value = false;
}

// While the overlay is open the search input has focus, so the layout's focus
// guard (#39) suppresses the global '?' binding and the char would just type
// into the field. '?' is the help key — swallow it and close instead.
function onKeydown(event) {
    if (event.key === '?') {
        event.preventDefault();
        close();
    }
}

// Fresh search + focus each time it opens.
watch(open, (isOpen) => {
    if (!isOpen) return;
    query.value = '';
    nextTick(() => input.value?.focus());
});
</script>

<template>
    <div
        v-if="open"
        class="fixed inset-0 z-50 flex items-start justify-center bg-black/20 pt-[12vh]"
        @mousedown.self="close"
        @keydown.esc.prevent="close"
        @keydown="onKeydown"
    >
        <div class="w-[440px] max-w-[90vw] overflow-hidden rounded-[18px] border border-card-border bg-surface shadow-card">
            <div class="border-b border-border-soft p-3">
                <input
                    ref="input"
                    v-model="query"
                    type="text"
                    placeholder="Search shortcuts…"
                    class="w-full rounded-[10px] border border-[#e0dbd0] bg-white px-3 py-2 text-[13px] text-ink outline-none placeholder:text-faint-3 focus:border-accent-tint-2"
                />
            </div>

            <div class="max-h-[60vh] overflow-y-auto p-3">
                <div v-if="navRows.length" class="mb-3">
                    <div class="mb-1.5 font-mono text-[10px] font-semibold uppercase tracking-[0.12em] text-faint">
                        Navigation
                    </div>
                    <div v-for="row in navRows" :key="row.label" class="flex items-center justify-between gap-4 py-1.5">
                        <span class="text-[13px] text-ink">{{ row.label }}</span>
                        <span class="flex gap-1">
                            <kbd
                                v-for="(token, i) in row.keys"
                                :key="i"
                                class="rounded-[6px] bg-sunken px-2 py-0.5 font-mono text-[11px] font-semibold text-muted"
                            >{{ token }}</kbd>
                        </span>
                    </div>
                </div>

                <div v-for="group in groups" :key="group.scope" class="mb-3 last:mb-0">
                    <div class="mb-1.5 font-mono text-[10px] font-semibold uppercase tracking-[0.12em] text-faint">
                        {{ group.scope }}
                    </div>
                    <div
                        v-for="action in group.actions"
                        :key="action.id"
                        class="flex items-center justify-between gap-4 py-1.5"
                    >
                        <span class="text-[13px] text-ink">{{ action.label }}</span>
                        <span class="flex gap-1">
                            <kbd
                                v-for="(token, i) in action.keys.split(/[\s+]/)"
                                :key="i"
                                class="rounded-[6px] bg-sunken px-2 py-0.5 font-mono text-[11px] font-semibold text-muted"
                            >{{ token }}</kbd>
                        </span>
                    </div>
                </div>
                <div v-if="!groups.length && !navRows.length" class="py-2 text-[13px] text-faint">No matching shortcuts.</div>
            </div>
        </div>
    </div>
</template>

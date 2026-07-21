<script setup>
import { computed, nextTick, ref } from 'vue';

// Searchable multi-select filter dropdown (notes page). `data-kb-ignore` keeps
// page keybindings suppressed while interacting; Escape closes locally.
const props = defineProps({
    modelValue: { type: Array, default: () => [] },
    options: { type: Array, default: () => [] }, // [{ value, label }]
    placeholder: { type: String, required: true }, // e.g. "All clients"
});
const emit = defineEmits(['update:modelValue']);

const open = ref(false);
const search = ref('');
const root = ref(null);
const searchInput = ref(null);

const filtered = computed(() => {
    const q = search.value.trim().toLowerCase();
    return q ? props.options.filter((o) => o.label.toLowerCase().includes(q)) : props.options;
});

const summary = computed(() => {
    const picked = props.options.filter((o) => props.modelValue.includes(o.value));
    if (!picked.length) return props.placeholder;
    return picked.length === 1 ? picked[0].label : `${picked[0].label} +${picked.length - 1}`;
});

function toggleOpen() {
    if (open.value) return close();
    open.value = true;
    search.value = '';
    nextTick(() => searchInput.value?.focus());
}

// Blur on close: focus parked inside the component keeps `data-kb-ignore`
// suppressing every page keybinding until it leaves.
function close() {
    open.value = false;
    if (root.value?.contains(document.activeElement)) document.activeElement.blur();
}

function toggle(value) {
    emit(
        'update:modelValue',
        props.modelValue.includes(value)
            ? props.modelValue.filter((v) => v !== value)
            : [...props.modelValue, value],
    );
}

function onFocusout(event) {
    if (!root.value?.contains(event.relatedTarget)) open.value = false;
}
</script>

<template>
    <div ref="root" class="relative" data-kb-ignore @keydown.esc.stop="close" @focusout="onFocusout">
        <button
            type="button"
            class="flex items-center gap-1.5 rounded-xl border border-card-border bg-surface px-3 py-2 text-[13px] outline-none transition focus:border-accent"
            :class="modelValue.length ? 'text-ink' : 'text-faint'"
            @click="toggleOpen"
        >
            {{ summary }}
            <svg class="h-3 w-3 text-faint-3" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5">
                <path d="M3 4.5 6 7.5 9 4.5" />
            </svg>
        </button>

        <div
            v-if="open"
            class="absolute left-0 top-full z-20 mt-1 w-[240px] rounded-xl border border-card-border bg-surface p-1.5 shadow-lg"
        >
            <input
                ref="searchInput"
                v-model="search"
                type="search"
                placeholder="Search…"
                class="mb-1 w-full rounded-lg border border-card-border bg-surface px-2.5 py-1.5 text-[13px] outline-none focus:border-accent"
            />
            <div class="max-h-[240px] overflow-y-auto">
                <button
                    v-for="o in filtered"
                    :key="o.value"
                    type="button"
                    class="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-[13px] hover:bg-surface-soft"
                    @click="toggle(o.value)"
                >
                    <span
                        class="flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded border text-[9px] leading-none"
                        :class="modelValue.includes(o.value) ? 'border-accent bg-accent text-white' : 'border-card-border'"
                    >
                        {{ modelValue.includes(o.value) ? '✓' : '' }}
                    </span>
                    {{ o.label }}
                </button>
                <p v-if="!filtered.length" class="px-2.5 py-1.5 text-[13px] text-faint">No matches</p>
            </div>
        </div>
    </div>
</template>

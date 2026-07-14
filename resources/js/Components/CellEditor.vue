<script setup>
import { computed, ref, watch } from 'vue';

// Popover editor for a day or a dragged range (ADR-0010). Anchored near the
// selection; picks type / hours / reason / planned↔confirmed, saves or deletes.
const props = defineProps({
    open: { type: Boolean, default: false },
    x: { type: Number, default: 0 },
    y: { type: Number, default: 0 },
    start: { type: String, default: '' },
    end: { type: String, default: '' },
    existing: { type: Object, default: null },
});
const emit = defineEmits(['save', 'delete', 'close']);

const TYPES = [
    { value: 'off', label: 'Off' },
    { value: 'sick', label: 'Sick' },
    { value: 'holiday', label: 'Holiday' },
    { value: 'extra', label: 'Extra' },
];

const type = ref('off');
const hours = ref(8);
const reason = ref('');
const status = ref('planned');

// Seed the form each time the popover opens.
watch(
    () => props.open,
    (isOpen) => {
        if (!isOpen) return;
        const e = props.existing;
        type.value = e ? e.type : 'off';
        hours.value = e ? Math.abs(Number(e.hours)) : 8;
        reason.value = e?.reason ?? '';
        status.value = e ? e.status : 'planned';
    },
);

const isRange = computed(() => props.start !== props.end && type.value !== 'extra');
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
function label(d) {
    const [y, m, day] = d.split('-').map(Number);
    return day + ' ' + MONTHS[m - 1];
}
const rangeLabel = computed(() =>
    isRange.value ? label(props.start) + ' – ' + label(props.end) : label(props.start),
);

// Clamp so the ~300×360 panel stays on-screen.
const style = computed(() => ({
    left: Math.min(props.x, (typeof window !== 'undefined' ? window.innerWidth : 1280) - 312) + 'px',
    top: Math.min(props.y + 8, (typeof window !== 'undefined' ? window.innerHeight : 900) - 372) + 'px',
}));

function save() {
    const h = Number(String(hours.value).replace(',', '.'));
    if (!h || h <= 0) return;
    const payload = { type: type.value, hours: h, status: status.value, reason: reason.value };
    if (props.existing) {
        payload.id = props.existing.id;
    } else {
        payload.start = props.start;
        if (isRange.value) payload.end = props.end;
    }
    emit('save', payload);
}
</script>

<template>
    <div v-if="open" class="fixed inset-0 z-40" @mousedown.self="emit('close')">
        <div
            class="fixed z-50 w-[300px] rounded-[18px] border border-card-border bg-surface p-[18px] shadow-card"
            :style="style"
        >
            <div class="mb-3.5 flex items-center justify-between">
                <div class="font-mono text-[11px] font-semibold uppercase tracking-[0.12em] text-faint">
                    {{ existing ? 'Edit day' : 'Add' }}
                </div>
                <div class="font-mono text-[12px] font-medium tabular-nums text-muted">{{ rangeLabel }}</div>
            </div>

            <!-- type -->
            <div class="mb-3.5 flex gap-0.5 rounded-[12px] bg-sunken p-1">
                <button
                    v-for="t in TYPES"
                    :key="t.value"
                    class="flex-1 cursor-pointer whitespace-nowrap rounded-[9px] py-2 text-[11.5px] font-semibold transition"
                    :class="type === t.value ? 'bg-surface text-ink shadow-[0_2px_6px_-2px_rgba(42,41,38,0.14)]' : 'text-[#8a8578]'"
                    @click="type = t.value"
                >
                    {{ t.label }}
                </button>
            </div>

            <!-- hours + status -->
            <div class="mb-3.5 grid grid-cols-2 gap-3">
                <label class="block">
                    <span class="mb-1.5 block font-mono text-[10px] font-semibold uppercase tracking-[0.1em] text-faint">Hours</span>
                    <input
                        v-model="hours"
                        type="text"
                        inputmode="decimal"
                        class="w-full rounded-[10px] border border-[#e0dbd0] bg-white px-3 py-2 font-mono text-[14px] font-semibold tabular-nums text-ink outline-none focus:border-accent-tint-2"
                    />
                </label>
                <div>
                    <span class="mb-1.5 block font-mono text-[10px] font-semibold uppercase tracking-[0.1em] text-faint">Status</span>
                    <div class="flex gap-0.5 rounded-[10px] bg-sunken p-0.5">
                        <button
                            class="flex-1 cursor-pointer rounded-[7px] py-[7px] text-[11.5px] font-semibold transition"
                            :class="status === 'planned' ? 'bg-surface text-ink shadow-[0_2px_6px_-2px_rgba(42,41,38,0.14)]' : 'text-[#8a8578]'"
                            @click="status = 'planned'"
                        >
                            Planned
                        </button>
                        <button
                            class="flex-1 cursor-pointer rounded-[7px] py-[7px] text-[11.5px] font-semibold transition"
                            :class="status === 'confirmed' ? 'bg-surface text-ink shadow-[0_2px_6px_-2px_rgba(42,41,38,0.14)]' : 'text-[#8a8578]'"
                            @click="status = 'confirmed'"
                        >
                            Confirmed
                        </button>
                    </div>
                </div>
            </div>

            <!-- reason / trip name -->
            <label class="mb-4 block">
                <span class="mb-1.5 block font-mono text-[10px] font-semibold uppercase tracking-[0.1em] text-faint">
                    Reason <span class="normal-case tracking-normal text-faint-4">· trip name</span>
                </span>
                <input
                    v-model="reason"
                    type="text"
                    :placeholder="type === 'extra' ? 'Agreed extra day…' : type === 'holiday' ? 'Kingsday…' : 'Egypte met familie…'"
                    class="w-full rounded-[10px] border border-[#e0dbd0] bg-white px-3 py-2 text-[13px] text-ink outline-none placeholder:text-faint-3 focus:border-accent-tint-2"
                    @keydown.enter="save"
                />
            </label>

            <div class="flex items-center gap-2">
                <button
                    v-if="existing"
                    class="cursor-pointer rounded-[10px] border border-transparent px-3 py-2.5 text-[13px] font-semibold text-[#b5877a] transition hover:border-[#eccaca] hover:bg-[#fbf2f1]"
                    @click="emit('delete', existing.id)"
                >
                    Delete
                </button>
                <div class="flex-1"></div>
                <button
                    class="cursor-pointer px-3 py-2.5 text-[13px] font-semibold text-faint-2 transition hover:text-muted"
                    @click="emit('close')"
                >
                    Cancel
                </button>
                <button
                    class="cursor-pointer rounded-[11px] bg-accent px-[18px] py-2.5 text-[13px] font-semibold text-white shadow-btn"
                    @click="save"
                >
                    Save
                </button>
            </div>
        </div>
    </div>
</template>

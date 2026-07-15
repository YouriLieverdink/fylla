<script setup>
defineProps({
    project: { type: Object, required: true },
});
defineEmits(['toggle-billable']);
</script>

<template>
    <div class="grid grid-cols-[1fr_auto] items-center gap-3 border-t border-divider-soft px-2 py-3">
        <div class="min-w-0">
            <div class="truncate text-[14px] font-medium">{{ project.name }}</div>
            <div v-if="project.code" class="mt-[3px] font-mono text-[11px] text-faint-3">{{ project.code }}</div>
        </div>
        <div class="flex items-center gap-4">
            <label class="flex cursor-pointer items-center gap-2 font-mono text-[11px] uppercase tracking-[0.1em] text-faint-3">
                <input
                    type="checkbox"
                    class="h-4 w-4 accent-accent"
                    :checked="project.billable"
                    @change="$emit('toggle-billable', $event.target.checked)"
                />
                Billable
            </label>
            <slot name="action" />
        </div>
    </div>
</template>

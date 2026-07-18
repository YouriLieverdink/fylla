import { onUnmounted, ref, watch } from 'vue';

// How many blocking modals are open (edit / promote-pick / manual-pick / ad-hoc
// / add-project / `?`). The layout's global key listener early-returns while
// this is > 0, so no page-local / cursor (j/k) / global binding fires beneath
// the scrim — regardless of focus. Modals register nothing in the action
// registry: native Escape/Enter/Tab only, and Escape is the sole exit (#43).
export const openModalCount = ref(0);

// Track one modal's open state into the shared counter. Pass a getter that is
// truthy while the modal is open (`() => editing.value !== null`). Cleans up on
// unmount so a modal open during page nav can't leave the counter stuck.
//
// Single-layer invariant: exactly one blocking modal at a time — nested modals
// would let a keystroke target a hidden layer. `CellEditor`'s inline popover is
// excluded (it stays on the #30 focus-guard / `data-kb-ignore` path).
export function useModalGuard(isOpen) {
    let counted = false;
    const set = (open) => {
        open = !!open;
        if (open === counted) return;
        counted = open;
        openModalCount.value += open ? 1 : -1;
        if (openModalCount.value > 1) {
            throw new Error(`useModalGuard: ${openModalCount.value} modals open — single-layer invariant broken`);
        }
    };
    watch(isOpen, set);
    onUnmounted(() => set(false));
}

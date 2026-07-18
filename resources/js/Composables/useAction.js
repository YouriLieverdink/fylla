import { onMounted, onUnmounted, reactive } from 'vue';

// Central reactive registry of live actions, keyed by id. The persistent
// layout watches this and rebinds tinykeys whenever it changes (#38).
//
// Bound-only (#33 binding rule, #28): an action enters the registry only if it
// earns a key. Unkeyed actions are unsupported — no command palette, no
// keyboardless fallback.
export const registry = reactive(new Map());

export function registerAction(action) {
    registry.set(action.id, action);
}

export function unregisterAction(id) {
    registry.delete(id);
}

// Declare an action inline in any component. Auto-registers on mount, drops on
// unmount. Envelope: {id, label, keys, scope, run}.
//
// `scope` is carried but unused for now — only global actions exist; scope
// filtering lands with the focus/targeting slice.
// ponytail: last writer wins on key collision — no conflict handling until two
// real actions actually clash.
export function useAction(action) {
    if (!action.keys) {
        throw new Error(`useAction("${action.id}"): keys required — the registry is bound-only`);
    }
    onMounted(() => registerAction(action));
    onUnmounted(() => unregisterAction(action.id));
}

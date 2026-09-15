import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';

vi.mock('@inertiajs/vue3', () => ({ router: { post: vi.fn() } }));

import TimerStack from './TimerStack.vue';

const overlap = {
    minutes: 15,
    earlier: { key: 'A-1', from: '14:00', to: '15:00' },
    later: { key: 'B-1', from: '14:45', to: null },
};

describe('TimerStack double-booked warning', () => {
    it('names the minutes and both segments when the day overlaps', () => {
        const w = mount(TimerStack, { props: { active: null, paused: [], overlaps: [overlap] } });

        const text = w.text();
        expect(text).toContain('15 min double-booked');
        expect(text).toContain('A-1');
        expect(text).toContain('14:00');
        expect(text).toContain('B-1');
        expect(text).toContain('14:45');
    });

    it('says nothing when there is no overlap', () => {
        const w = mount(TimerStack, { props: { active: null, paused: [], overlaps: [] } });

        expect(w.text()).not.toContain('double-booked');
    });
});

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { nextTick } from 'vue';
import { shallowMount } from '@vue/test-utils';

import Utilization from './Utilization.vue';
import { registry } from '../Composables/useAction';

// jsdom implements neither; the cursor's scroll-into-view watcher would throw.
Element.prototype.scrollIntoView = vi.fn();
window.scrollTo = vi.fn();

const report = { weeks: [], totals: {}, target: 90, softFloor: 80 };
const run = (id) => registry.get(id).run();

const projection = {
    thisWeek: {
        capacityHours: 32, remainingHours: 8, loggedBillableHours: 10,
        floor: { neededTotal: 20.25, neededMore: 10.25, feasible: false },
        target: { neededTotal: 22, neededMore: 12, feasible: false },
        headroomHours: 0,
    },
    paceHours: 24.8,
    paceWeeks: 4,
    sustained: {
        horizonWeeks: 4, effectiveWeeks: 4,
        floor: { hoursPerWeek: 26.5, feasible: true },
        target: { hoursPerWeek: 28, feasible: true },
    },
    timeToBand: { weeksToFloor: 3, weeksToTarget: null, capWeeks: 26 },
};

// The stat reads the two in-band states off the window's own ratio.
const banded = (utilization) => ({ ...report, totals: { utilization } });

describe('Utilization view-switcher keyset (#45)', () => {
    beforeEach(() => { registry.clear(); });

    it('w/p/t switch the active view', async () => {
        const w = shallowMount(Utilization, { props: { report } });
        await nextTick();

        run('util:projects');
        expect(w.vm.view).toBe('By project');
        run('util:entries');
        expect(w.vm.view).toBe('Time entries');
        run('util:weekly');
        expect(w.vm.view).toBe('Weekly breakdown');

        w.unmount();
    });

    it('registers under the utilization scope', async () => {
        const w = shallowMount(Utilization, { props: { report } });
        await nextTick();
        const keys = [...registry.values()].filter((a) => a.scope === 'utilization').map((a) => a.keys).sort();
        expect(keys).toEqual(['p', 't', 'w']);
        w.unmount();
    });
});

describe('hours-needed-this-week card (#105)', () => {
    beforeEach(() => { registry.clear(); });

    it('is hidden when the projection is null', () => {
        const w = shallowMount(Utilization, { props: { report } });
        expect(w.find('[data-card="projection"]').exists()).toBe(false);
        w.unmount();
    });

    it('renders below the floor with the out-of-reach copy', () => {
        const w = shallowMount(Utilization, { props: { report, projection } });
        expect(w.find('[data-card="projection"]').exists()).toBe(true);
        expect(w.vm.prescription.value).toBe('10.25h');
        expect(w.vm.prescription.caption).toContain('out of reach this week');
        w.unmount();
    });

    it('reads headroom when both thresholds are already met', () => {
        const clear = { thisWeek: { ...projection.thisWeek,
            floor: { neededTotal: 12, neededMore: 0, feasible: true },
            target: { neededTotal: 14, neededMore: 0, feasible: true },
            headroomHours: 4.5 } };
        const w = shallowMount(Utilization, { props: { report, projection: clear } });
        expect(w.vm.prescription.value).toBe('+4.5h');
        expect(w.vm.prescription.caption).toContain('4.5h to spare');
        w.unmount();
    });
});

describe('sustained stat (#107)', () => {
    it('renders the four-week range below the band', () => {
        const w = shallowMount(Utilization, { props: { report: banded(60), projection } });
        expect(w.vm.sustained).toBe('26.5–28h/wk for 4 weeks');
        w.unmount();
    });

    it('uses the current pace once clear of the band', () => {
        const w = shallowMount(Utilization, { props: { report: banded(94), projection: { ...projection, paceHours: 26.9 } } });
        expect(w.vm.sustained).toBe('26.9h/wk (current pace)');
        w.unmount();
    });

    it('says out of reach when the four-week solve is infeasible', () => {
        const unreachable = { ...projection, sustained: {
            ...projection.sustained,
            target: { hoursPerWeek: 32, feasible: false },
        } };
        const w = shallowMount(Utilization, { props: { report: banded(60), projection: unreachable } });
        expect(w.vm.sustained).toBe('out of reach');
        w.unmount();
    });
});

describe('time-to-band stat (#106)', () => {
    const read = (props) => {
        const w = shallowMount(Utilization, { props });
        const value = w.vm.timeToBand;
        w.unmount();

        return value;
    };

    it('counts the weeks to the floor at the current pace', () => {
        expect(read({ report: banded(60), projection })).toBe('3 wks at 24.8h/wk');
    });

    it('reads holding inside the band and n/a above the target', () => {
        expect(read({ report: banded(82), projection })).toBe('holding');
        expect(read({ report: banded(94), projection })).toBe('n/a');
    });

    it('says not at this pace when the band is never reached', () => {
        const stalled = { ...projection, paceHours: 19.8,
            timeToBand: { weeksToFloor: null, weeksToTarget: null, capWeeks: 26 } };
        expect(read({ report: banded(60), projection: stalled })).toBe('not at this pace');
    });

    it('is absent without a projection', () => {
        expect(read({ report })).toBe(null);
    });
});

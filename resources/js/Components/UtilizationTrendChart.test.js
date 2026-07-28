import { describe, expect, it } from 'vitest';
import { mount } from '@vue/test-utils';

import UtilizationTrendChart from './UtilizationTrendChart.vue';

const points = [
    { label: 'Jul 13', value: 72, billableShare: 80 },
    { label: 'Jul 20', value: 74, billableShare: 82 },
];

describe('UtilizationTrendChart', () => {
    it('shades and labels the soft-floor-to-target band', () => {
        const wrapper = mount(UtilizationTrendChart, {
            props: { points, floor: 73, target: 75 },
        });

        expect(wrapper.get('[data-band]').attributes('fill-opacity')).not.toBe('0');
        expect(wrapper.get('[data-floor]').attributes('stroke-dasharray')).toBeTruthy();
        const series = wrapper.get('[data-series="utilization"]').element;
        const label = wrapper.get('[data-floor-label]');
        expect(label.text()).toBe('73% floor');
        expect(series.compareDocumentPosition(label.element) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
        expect(wrapper.text()).toContain('73–75% band');
    });
});

import { describe, expect, it } from 'vitest';
import { mount } from '@vue/test-utils';

import ProjectionChart from './ProjectionChart.vue';

const history = [
    { label: 'Jun 29', value: 68 },
    { label: 'Jul 6', value: 71 },
    { label: 'Jul 13', value: 72 },
];
const projection = [
    { label: 'Jul 13', value: 73 },
    { label: 'Jul 20', value: 74 },
    { label: 'Jul 27', value: 75 },
    { label: 'Aug 3', value: 76 },
];

describe('ProjectionChart', () => {
    it('joins history to a separately supplied dashed continuation', () => {
        const wrapper = mount(ProjectionChart, { props: { history, projection } });
        const historyPath = wrapper.get('[data-series="history"]');
        const projectionPath = wrapper.get('[data-series="projection"]');

        expect(historyPath.attributes('d')).toMatch(/^M /);
        expect(projectionPath.attributes('d')).toContain(historyPath.attributes('d').split(' L ').at(-1));
        expect(projectionPath.attributes('stroke-dasharray')).toBeTruthy();
        expect(wrapper.findAll('[data-forward-point]')).toHaveLength(4);
    });

    it('breaks both lines at null weeks instead of plotting zero', () => {
        const wrapper = mount(ProjectionChart, {
            props: {
                history: [history[0], { label: 'Jul 6', value: null }, history[2]],
                projection: [projection[0], { label: 'Jul 20', value: null }, projection[2]],
            },
        });

        expect(wrapper.get('[data-series="history"]').attributes('d').match(/M /g)).toHaveLength(2);
        expect(wrapper.get('[data-series="projection"]').attributes('d').match(/M /g)).toHaveLength(2);
        expect(wrapper.findAll('[data-history-point]')).toHaveLength(2);
        expect(wrapper.findAll('[data-forward-point]')).toHaveLength(2);
        expect(wrapper.findAll('[data-null-week]')).toHaveLength(2);
    });

    it('shades the floor-to-target band and labels its dashed floor edge', () => {
        const wrapper = mount(ProjectionChart, {
            props: { history, projection, floor: 73, target: 75 },
        });

        expect(wrapper.get('[data-band]').attributes('fill-opacity')).not.toBe('0');
        expect(wrapper.get('[data-floor]').attributes('stroke-dasharray')).toBeTruthy();
        expect(wrapper.get('[data-floor-label]').text()).toBe('73% floor');
        expect(wrapper.find('[data-tooltip]').exists()).toBe(false);
    });
});

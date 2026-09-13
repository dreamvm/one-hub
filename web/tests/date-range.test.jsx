import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act, within } from '@testing-library/react';
import dayjs from 'dayjs';
import DateRangePicker from '../src/ui-component/DateRangePicker';
import { Wrapper } from './ui-test-utils';

const defaults = () => ({ start: dayjs('2024-01-05').startOf('day'), end: dayjs('2024-01-12').endOf('day') });
const labels = { start: '开始时间', end: '结束时间' };
beforeEach(() => {
  // Exercise desktop DatePicker; browser QA also covers narrow mobile layouts.
  vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
    matches: true,
    media: query,
    addListener() {},
    removeListener() {},
    addEventListener() {},
    removeEventListener() {}
  }));
});

describe('date range field identity', () => {
  it.each(['field', 'calendar-button'])('opens only the end picker via %s and preserves start date', async (entry) => {
    const onChange = vi.fn();
    render(<DateRangePicker defaultValue={defaults()} localeText={labels} onChange={onChange} />, { wrapper: Wrapper });
    const endField = screen.getByRole('textbox', { name: '结束时间' });
    if (entry === 'field') fireEvent.click(endField);
    else fireEvent.click(within(endField.closest('.MuiFormControl-root')).getByRole('button'));
    expect(screen.getByRole('dialog', { name: '结束时间' })).toBeTruthy();
    expect(screen.queryByRole('dialog', { name: '开始时间' })).toBeNull();
    fireEvent.click(screen.getByRole('gridcell', { name: '10', exact: true }));
    await waitFor(() => expect(onChange).toHaveBeenCalledTimes(1));
    expect(screen.getByRole('textbox', { name: '开始时间' }).value).toBe('2024/01/05');
    expect(endField.value).toBe('2024/01/10');
    expect(onChange.mock.calls[0][0].start.valueOf()).toBe(defaults().start.valueOf());
    expect(onChange.mock.calls[0][0].end.format('YYYY-MM-DD HH:mm:ss')).toBe('2024-01-10 23:59:59');
  });

  it('preserves the start-then-end selection flow', async () => {
    const onChange = vi.fn();
    render(<DateRangePicker defaultValue={defaults()} localeText={labels} onChange={onChange} />, { wrapper: Wrapper });
    fireEvent.click(screen.getByRole('textbox', { name: '开始时间' }));
    fireEvent.click(screen.getByRole('gridcell', { name: '6', exact: true }));
    const endDialog = await screen.findByRole('dialog', { name: '结束时间' });
    fireEvent.click(within(endDialog).getByRole('gridcell', { name: '11', exact: true }));
    await waitFor(() => expect(onChange).toHaveBeenCalledTimes(1));
    expect(onChange.mock.calls[0][0].start.format('YYYY-MM-DD HH:mm:ss')).toBe('2024-01-06 00:00:00');
    expect(onChange.mock.calls[0][0].end.format('YYYY-MM-DD HH:mm:ss')).toBe('2024-01-11 23:59:59');
  });

  it('ignores invalid or empty picker values instead of throwing or corrupting the range', async () => {
    const ref = React.createRef();
    render(<DateRangePicker ref={ref} defaultValue={defaults()} localeText={labels} />, { wrapper: Wrapper });
    await act(async () => {
      ref.current.handleStartChange(null);
      ref.current.handleEndChange(dayjs('invalid'));
    });
    expect(screen.getByRole('textbox', { name: '开始时间' }).value).toBe('2024/01/05');
    expect(screen.getByRole('textbox', { name: '结束时间' }).value).toBe('2024/01/12');
  });
});

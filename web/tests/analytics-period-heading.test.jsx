import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import dayjs from 'dayjs';
import { Wrapper, response, deferred } from './ui-test-utils';
import Overview from '../src/views/Analytics/component/Overview';
import { API } from '../src/utils/api';

vi.mock('../src/utils/api', () => ({ API: { get: vi.fn() } }));
vi.mock('@iconify/react', () => ({ Icon: () => <span /> }));
vi.mock('../src/ui-component/DateRangePicker', () => ({
  default: ({ onChange }) => (
    <>
      <button onClick={() => onChange({ start: dayjs('2026-01-10').startOf('day'), end: dayjs('2026-01-12').endOf('day') })}>
        Draft first period
      </button>
      <button onClick={() => onChange({ start: dayjs('2026-01-20').startOf('day'), end: dayjs('2026-01-22').endOf('day') })}>
        Draft second period
      </button>
    </>
  )
}));
vi.mock('../src/ui-component/chart/ApexCharts', () => ({
  default: ({ isLoading, chartDatas }) => (
    <section data-testid="chart" data-loading={String(isLoading)}>
      {JSON.stringify(chartDatas)}
    </section>
  )
}));
const first = '2026-01-10 - 2026-01-12';
const second = '2026-01-20 - 2026-01-22';
const fixture = (params) => ({
  channel_statistics: [
    {
      Date: dayjs.unix(params.start_timestamp).format('YYYY-MM-DD'),
      Channel: 'synthetic-channel',
      Quota: 500000,
      PromptTokens: 4,
      CompletionTokens: 2,
      RequestCount: 1,
      RequestTime: 1000
    }
  ]
});
const finished = () => screen.getAllByTestId('chart').forEach((e) => expect(e.dataset.loading).toBe('false'));
beforeEach(() => {
  vi.clearAllMocks();
  API.get.mockImplementation(async (url, { params }) => response(fixture(params)));
});
async function mount(mode = 'light') {
  render(<Overview />, { wrapper: ({ children }) => <Wrapper mode={mode}>{children}</Wrapper> });
  await waitFor(finished);
}
describe('analytics queried period heading', () => {
  it.each(['light', 'dark'])('keeps the initial query heading aligned with its data in %s', async (mode) => {
    await mount(mode);
    const params = API.get.mock.calls[0][1].params;
    const title = `${dayjs.unix(params.start_timestamp).format('YYYY-MM-DD')} - ${dayjs.unix(params.end_timestamp).format('YYYY-MM-DD')}`;
    expect(screen.getByRole('heading', { name: title })).toBeTruthy();
    expect(screen.getAllByTestId('chart')[0].textContent).toContain('synthetic-channel');
  });
  it('does not relabel existing charts when only the draft period changes', async () => {
    await mount();
    const heading = screen.getByRole('heading').textContent;
    fireEvent.click(screen.getByRole('button', { name: 'Draft first period' }));
    expect(API.get).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('heading').textContent).toBe(heading);
    expect(screen.getAllByTestId('chart')[0].textContent).toContain('synthetic-channel');
  });
  it('keeps a pending and completed search labelled with its submitted period', async () => {
    await mount();
    const pending = deferred();
    API.get.mockImplementationOnce(() => pending.promise);
    fireEvent.click(screen.getByRole('button', { name: 'Draft first period' }));
    fireEvent.click(screen.getByRole('button', { name: '搜索', exact: true }));
    expect(screen.getByRole('heading', { name: first })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Draft second period' }));
    expect(API.get).toHaveBeenCalledTimes(2);
    expect(screen.getByRole('heading').textContent).toBe(first);
    screen.getAllByTestId('chart').forEach((e) => expect(e.dataset.loading).toBe('true'));
    await act(async () => pending.resolve(response(fixture(API.get.mock.calls[1][1].params))));
    expect(screen.getByRole('heading').textContent).toBe(first);
    expect(screen.getAllByTestId('chart')[0].textContent).toContain('2026-01-10');
    finished();
  });
  it('keeps the latest submitted period when an older search completes last', async () => {
    await mount();
    const old = deferred();
    API.get.mockImplementationOnce(() => old.promise);
    fireEvent.click(screen.getByRole('button', { name: 'Draft first period' }));
    fireEvent.click(screen.getByRole('button', { name: '搜索', exact: true }));
    fireEvent.click(screen.getByRole('button', { name: 'Draft second period' }));
    await act(async () => fireEvent.click(screen.getByRole('button', { name: '搜索', exact: true })));
    expect(screen.getByRole('heading').textContent).toBe(second);
    await act(async () => old.resolve(response(fixture(API.get.mock.calls[1][1].params))));
    expect(screen.getByRole('heading').textContent).toBe(second);
    expect(screen.getAllByTestId('chart')[0].textContent).toContain('2026-01-20');
  });
});

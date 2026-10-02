import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import dayjs from 'dayjs';
import { Wrapper, response, deferred } from './ui-test-utils';
import Overview from '../src/views/Analytics/component/Overview';
import { API } from '../src/utils/api';
import { showError } from '../src/utils/common';
vi.mock('../src/utils/api', () => ({ API: { get: vi.fn() } }));
vi.mock('../src/utils/common', async (original) => ({ ...(await original()), showError: vi.fn() }));
vi.mock('@iconify/react', () => ({ Icon: () => <span /> }));
vi.mock('../src/ui-component/chart/ApexCharts', () => ({
  default: ({ isLoading, chartDatas, title }) => (
    <section data-testid="chart" data-loading={String(isLoading)} aria-label={title}>
      {JSON.stringify(chartDatas)}
    </section>
  )
}));
const fixture = (params, marker = 'synthetic-normal') => {
  const date = dayjs.unix(params.start_timestamp).format('YYYY-MM-DD');
  return {
    user_statistics: [{ date, user_count: 3, inviter_user_count: 1 }],
    channel_statistics: [
      { Date: date, Channel: marker, Quota: 500000, PromptTokens: 4, CompletionTokens: 2, RequestCount: 1, RequestTime: 1000 }
    ],
    redemption_statistics: [{ date, quota: 500000, user_count: 1 }],
    order_statistics: [{ date, order_amount: 10 }]
  };
};
const charts = () => screen.getAllByTestId('chart');
const expectFinished = () => charts().forEach((chart) => expect(chart.dataset.loading).toBe('false'));
beforeEach(() => {
  vi.clearAllMocks();
  API.get.mockImplementation(async (url, { params }) => response(fixture(params)));
});
function mount(mode = 'light') {
  return render(<Overview />, { wrapper: ({ children }) => <Wrapper mode={mode}>{children}</Wrapper> });
}
describe('analytics overview request lifecycle', () => {
  it.each(['light', 'dark'])('loads all charts with normal period data in %s', async (mode) => {
    mount(mode);
    await waitFor(expectFinished);
    expect(charts()).toHaveLength(6);
    expect(charts()[0].textContent).toContain('synthetic-normal');
    expect(charts()[1].textContent).toContain('总Tokens：6');
    expect(charts()[4].textContent).toContain('总注册人数：3');
    expect(charts()[5].textContent).toContain('总充值数：10');
    expect(API.get).toHaveBeenCalledTimes(1);
    expect(API.get.mock.calls[0][1].params).toMatchObject({ group_type: 'model_type', user_id: 0 });
  });
  it.each(['rejection', 'undefined'])('finishes every chart on current %s', async (mode) => {
    API.get.mockImplementation(async () => {
      if (mode === 'rejection') throw new Error('Synthetic unavailable');
      return undefined;
    });
    await act(async () => mount());
    expectFinished();
  });
  it('keeps the latest search result when an older response arrives last', async () => {
    const first = deferred();
    API.get.mockImplementationOnce(() => first.promise);
    mount();
    fireEvent.change(screen.getByRole('spinbutton', { name: '用户ID' }), { target: { value: '202' } });
    API.get.mockImplementationOnce(async (url, { params }) => response(fixture(params, 'synthetic-current')));
    await act(async () => fireEvent.click(screen.getByRole('button', { name: '搜索', exact: true })));
    expect(charts()[0].textContent).toContain('synthetic-current');
    await act(async () => first.resolve(response(fixture(API.get.mock.calls[0][1].params, 'synthetic-obsolete'))));
    expect(charts()[0].textContent).toContain('synthetic-current');
    expectFinished();
    expect(API.get.mock.calls[1][1].params.user_id).toBe(202);
  });
  it('ignores an obsolete business error after unmount', async () => {
    const pending = deferred();
    API.get.mockReturnValue(pending.promise);
    const view = mount();
    view.unmount();
    await act(async () => pending.resolve({ data: { success: false, message: 'Synthetic obsolete failure' } }));
    expect(showError).not.toHaveBeenCalled();
  });
  it('replaces previous charts when the next successful response has no data', async () => {
    mount();
    await waitFor(expectFinished);
    API.get.mockResolvedValueOnce(response(null));
    await act(async () => fireEvent.click(screen.getByRole('button', { name: '搜索', exact: true })));
    expectFinished();
    charts().forEach((chart) => expect(chart.textContent).not.toContain('synthetic-normal'));
  });
  it('keeps all charts loading while the latest request is pending despite an old rejection', async () => {
    const old = deferred(),
      current = deferred();
    API.get.mockImplementationOnce(() => old.promise).mockImplementationOnce(() => current.promise);
    mount();
    fireEvent.click(screen.getByRole('button', { name: '搜索', exact: true }));
    await act(async () => old.reject(new Error('Synthetic obsolete rejection')));
    charts().forEach((chart) => expect(chart.dataset.loading).toBe('true'));
    await act(async () => current.resolve(response(fixture(API.get.mock.calls[1][1].params, 'synthetic-current'))));
    expectFinished();
    expect(charts()[0].textContent).toContain('synthetic-current');
  });
  it('clears old data on a current business failure and recovers with a new search', async () => {
    mount();
    await waitFor(expectFinished);
    API.get.mockResolvedValueOnce({ data: { success: false, message: 'Synthetic current failure' } });
    await act(async () => fireEvent.click(screen.getByRole('button', { name: '搜索', exact: true })));
    expectFinished();
    expect(showError).toHaveBeenCalledExactlyOnceWith('Synthetic current failure');
    charts().forEach((chart) =>
      expect(chart.textContent).toBe(chart === charts()[0] || chart === charts()[1] || chart === charts()[2] ? '{}' : 'null')
    );
    await act(async () => fireEvent.click(screen.getByRole('button', { name: '搜索', exact: true })));
    expectFinished();
    expect(charts()[0].textContent).toContain('synthetic-normal');
  });
  it('does not leave partially updated charts when a response cannot be transformed', async () => {
    API.get.mockImplementation(async (url, { params }) => response({ ...fixture(params), order_statistics: {} }));
    await act(async () => mount());
    expectFinished();
    charts().forEach((chart) => expect(chart.textContent).not.toContain('synthetic-normal'));
  });
  it('rejects stale first-mount data under StrictMode', async () => {
    const pending = deferred();
    API.get.mockImplementationOnce(() => pending.promise);
    render(
      <React.StrictMode>
        <Overview />
      </React.StrictMode>,
      { wrapper: Wrapper }
    );
    await waitFor(expectFinished);
    await act(async () => pending.resolve(response(fixture(API.get.mock.calls[0][1].params, 'synthetic-obsolete'))));
    expect(charts()[0].textContent).toContain('synthetic-normal');
    expectFinished();
  });
});

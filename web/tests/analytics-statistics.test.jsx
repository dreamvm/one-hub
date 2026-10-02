import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { act, render, screen, waitFor } from '@testing-library/react';
import Statistics from '../src/views/Analytics/component/Statistics';
import { API } from '../src/utils/api';
import { Wrapper, deferred } from './ui-test-utils';

vi.mock('../src/utils/api', () => ({ API: { get: vi.fn() } }));
vi.mock('../src/utils/common', async (original) => ({ ...(await original()), showError: vi.fn() }));
const sample = () => ({
  user_statistics: { total_quota: 1000000, total_used_quota: 500000, total_user: 8, total_inviter_user: 3 },
  channel_statistics: [
    { status: 1, total_channels: 3 },
    { status: 2, total_channels: 2 }
  ],
  redemption_statistic: [{ quota: 500000 }],
  order_statistics: [{ quota: 1000000, money: 2, order_currency: 'USD' }]
});
beforeEach(() => {
  API.get.mockReset();
  localStorage.setItem('quota_per_unit', '500000');
  localStorage.setItem('display_in_currency', 'true');
});

describe('analytics statistics request lifecycle', () => {
  it.each(['light', 'dark'])('renders successful statistics in %s mode', async (mode) => {
    API.get.mockResolvedValue({ data: { success: true, data: sample() } });
    const { container } = render(
      <Wrapper mode={mode}>
        <Statistics />
      </Wrapper>
    );
    expect(await screen.findByRole('heading', { name: '$3.00' })).toBeTruthy();
    expect(screen.getByRole('heading', { name: '8' })).toBeTruthy();
    expect(screen.getByRole('heading', { name: '5' })).toBeTruthy();
    expect(container.textContent).toContain('USD: 2');
    expect(container.querySelector('.MuiSkeleton-root')).toBeNull();
  });

  it.each([
    ['rejection', () => Promise.reject(new Error('Synthetic unavailable'))],
    ['empty interceptor response', () => Promise.resolve(undefined)],
    ['application failure', () => Promise.resolve({ data: { success: false, message: 'Synthetic unavailable' } })]
  ])('ends loading and exposes failure for %s', async (_name, response) => {
    API.get.mockImplementation(response);
    const { container } = render(
      <Wrapper>
        <Statistics />
      </Wrapper>
    );
    await waitFor(() => expect(container.querySelector('.MuiSkeleton-root')).toBeNull());
    expect(screen.getByRole('alert')).toBeTruthy();
    expect(screen.queryByRole('heading')).toBeNull();
  });
  it('keeps the API payload numeric and unchanged', async () => {
    const data = sample();
    const original = structuredClone(data);
    API.get.mockResolvedValue({ data: { success: true, data } });
    render(
      <Wrapper>
        <Statistics />
      </Wrapper>
    );
    expect(await screen.findByRole('heading', { name: '$3.00' })).toBeTruthy();
    expect(data).toEqual(original);
  });

  it('renders successful empty series as zero instead of loading or error', async () => {
    API.get.mockResolvedValue({
      data: { success: true, data: { user_statistics: null, channel_statistics: null, redemption_statistic: null, order_statistics: null } }
    });
    const { container } = render(
      <Wrapper>
        <Statistics />
      </Wrapper>
    );
    await waitFor(() => expect(container.querySelector('.MuiSkeleton-root')).toBeNull());
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.getAllByRole('heading', { name: '$0.00' })).toHaveLength(2);
    expect(screen.getAllByRole('heading', { name: '0' })).toHaveLength(2);
  });

  it('ignores an obsolete StrictMode result instead of accumulating formatted totals', async () => {
    const old = deferred();
    API.get.mockReturnValueOnce(old.promise).mockResolvedValueOnce({ data: { success: true, data: sample() } });
    const { container } = render(
      <React.StrictMode>
        <Wrapper>
          <Statistics />
        </Wrapper>
      </React.StrictMode>
    );
    expect(await screen.findByRole('heading', { name: '$3.00' })).toBeTruthy();
    const obsolete = sample();
    obsolete.order_statistics[0].quota = 3000000;
    await act(async () => old.resolve({ data: { success: true, data: obsolete } }));
    expect(screen.getByRole('heading', { name: '$3.00' })).toBeTruthy();
    expect(container.textContent.match(/USD: 2/g)).toHaveLength(1);
    expect(container.textContent).not.toContain('NaN');
    expect(API.get).toHaveBeenCalledTimes(2);
  });
});

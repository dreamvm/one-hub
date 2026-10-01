import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import Invoice from '../src/views/Invoice';
import { API } from '../src/utils/api';
import { Wrapper, zh } from './ui-test-utils';

const navigation = vi.hoisted(() => vi.fn());
// Decorative icons must not start remote loaders or outlive the isolated test DOM.
vi.mock('@iconify/react', () => ({ Icon: () => <span /> }));
vi.mock('react-router-dom', async (original) => ({ ...(await original()), useNavigate: () => navigation }));
vi.mock('../src/utils/api', () => ({ API: { get: vi.fn() } }));
vi.mock('../src/utils/common', async (original) => ({ ...(await original()), showError: vi.fn() }));
beforeEach(() => {
  navigation.mockReset();
  API.get.mockReset();
  localStorage.setItem('quota_per_unit', '500000');
});
function seed(date) {
  API.get.mockResolvedValue({
    data: {
      success: true,
      data: {
        total_count: 1,
        data: [{ id: 1, date, quota: 100, prompt_tokens: 12, completion_tokens: 7, request_count: 1, request_time: 1000 }]
      }
    }
  });
}
describe('invoice month navigation', () => {
  it.each(['2026-09-01', '2024-02-29'])('opens the expected month for valid date %s', async (date) => {
    seed(date);
    render(
      <Wrapper>
        <Invoice />
      </Wrapper>
    );
    fireEvent.click(await screen.findByRole('button', { name: zh.invoice_index.viewInvoice }));
    expect(navigation).toHaveBeenCalledWith(`/panel/invoice/detail/${date.slice(0, 7)}`);
  });
  it.each(['', '?qa=1', '#qa', '%', '../2026-09', '2026/09/01'])('does not navigate to a malformed detail route for %s', async (date) => {
    seed(date);
    render(
      <Wrapper>
        <Invoice />
      </Wrapper>
    );
    fireEvent.click(await screen.findByRole('button', { name: zh.invoice_index.viewInvoice }));
    expect(navigation).not.toHaveBeenCalled();
  });
});

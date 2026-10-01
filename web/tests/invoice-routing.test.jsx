import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Outlet, useLocation } from 'react-router-dom';
import { Provider } from 'react-redux';
import { ThemeProvider } from '@mui/material/styles';
import ThemeRoutes from '../src/routes';
import { store } from '../src/store';
import theme from '../src/themes';
import { API } from '../src/utils/api';
import { getInvoiceMonth, isInvoiceMonth } from '../src/utils/invoiceDate';
import './ui-test-utils';

vi.mock('../src/layout/MainLayout', () => ({ default: () => <Outlet /> }));
vi.mock('../src/layout/MinimalLayout', () => ({ default: () => <Outlet /> }));
vi.mock('../src/utils/api', () => ({ API: { get: vi.fn() } }));
vi.mock('../src/utils/common', async (original) => ({ ...(await original()), showError: vi.fn() }));
function LocationProbe() {
  return <output data-testid="location">{useLocation().pathname}</output>;
}
function renderPage(path) {
  return render(
    <Provider store={store}>
      <ThemeProvider theme={theme(store.getState().customization)}>
        <MemoryRouter initialEntries={[path]}>
          <ThemeRoutes />
          <LocationProbe />
        </MemoryRouter>
      </ThemeProvider>
    </Provider>
  );
}
beforeEach(() => {
  localStorage.setItem('quota_per_unit', '500000');
  API.get.mockReset().mockImplementation(async (url) => {
    if (url === '/api/user/invoice') return { data: { success: true, data: { total_count: 0, data: [] } } };
    if (url === '/api/user/invoice/detail') return { data: { success: true, data: [] } };
    if (url === '/api/user/self')
      return { data: { success: true, data: { id: 1, username: 'Synthetic user', email: 'qa@example.invalid' } } };
    throw new Error(`Unexpected fixture endpoint: ${url}`);
  });
});

describe('canonical invoice date values', () => {
  it.each(['2026-09', '2026-09-01', '2026-09-01 00:00:00', '2026-09-01T00:00:00Z', '2026-09-01T00:00:00.123456789+08:00'])(
    'preserves month for %s',
    (value) => expect(getInvoiceMonth(value)).toBe('2026-09')
  );
  it.each(['2024-02-29', '2000-02-29'])('accepts leap date %s', (value) => expect(getInvoiceMonth(value)).toBe(value.slice(0, 7)));
  it.each([
    null,
    undefined,
    123,
    {},
    '',
    '2026-13',
    '0000-01',
    '1900-02-29',
    '2025-02-29',
    '2026-04-31',
    '2026-09-00',
    '2026-09-01T25:00:00Z',
    '2026-09-01extra',
    '%',
    '../2026-09'
  ])('rejects malformed date %j', (value) => expect(getInvoiceMonth(value)).toBeNull());
  it('restricts route parameters to a whole canonical month', () => {
    expect(isInvoiceMonth('2026-09')).toBe(true);
    expect(isInvoiceMonth('2026-09-01')).toBe(false);
    expect(isInvoiceMonth('2026-9')).toBe(false);
  });
});

describe('actual invoice route configuration', () => {
  it.each([
    '/panel/invoice/detail',
    '/panel/invoice/detail/2026/09',
    '/panel/invoice/detail/2026-13',
    '/panel/invoice/detail/2026-09-01',
    '/panel/invoice/detail/2026%2F09'
  ])('recovers %s to invoice list without a detail request', async (path) => {
    renderPage(path);
    await waitFor(() => expect(screen.getByTestId('location').textContent).toBe('/panel/invoice'));
    expect(API.get.mock.calls.some(([url]) => url === '/api/user/invoice/detail')).toBe(false);
    expect(await screen.findByText('Invoice')).toBeTruthy();
  });
  it('keeps a valid month and sends the expected first-day query', async () => {
    renderPage('/panel/invoice/detail/2024-02');
    expect(await screen.findByText('Synthetic user')).toBeTruthy();
    expect(API.get).toHaveBeenCalledWith('/api/user/invoice/detail', { params: { date: '2024-02-01' } });
    expect(screen.getByTestId('location').textContent).toBe('/panel/invoice/detail/2024-02');
  });
  it.each(['/unmatched-synthetic-route', '/panel/unmatched-synthetic-route'])(
    'renders the existing not-found page for %s',
    async (path) => {
      const { container } = renderPage(path);
      expect(await screen.findByRole('button')).toBeTruthy();
      expect(container.querySelector('img')).not.toBeNull();
      expect(API.get).not.toHaveBeenCalled();
    }
  );
});

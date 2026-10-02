import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { Wrapper, response, deferred } from './ui-test-utils';
import { store } from '../src/store';
import { LOGIN, LOGOUT } from '../src/store/actions';
import { API } from '../src/utils/api';
import { showError } from '../src/utils/common';
import Token from '../src/views/Token';
import { UserContext } from '../src/contexts/UserContext';
const loadUserGroup = vi.fn();
function TokenList() {
  return (
    <UserContext.Provider value={{ loadUserGroup }}>
      <Token />
    </UserContext.Provider>
  );
}

vi.mock('../src/utils/api', () => ({ API: { get: vi.fn() } }));
vi.mock('../src/utils/common', async (original) => ({ ...(await original()), showError: vi.fn(), showSuccess: vi.fn() }));
vi.mock('@mui/material/useMediaQuery', () => ({ default: () => true }));
vi.mock('@iconify/react', () => ({ Icon: () => <span /> }));
// Real list, toolbar, pagination and effects; isolate unrelated row operations/editors.
vi.mock('../src/views/Token/component/TableRow', () => ({
  default: ({ item }) => (
    <tr>
      <td>{item.name}</td>
    </tr>
  )
}));
vi.mock('../src/views/Token/component/EditModal', () => ({ default: () => null }));

const rows = (name) => response({ data: [{ id: 1, name }], total_count: 30 });
let requests;
beforeEach(() => {
  vi.clearAllMocks();
  store.dispatch({ type: LOGOUT });
  requests = [];
  API.get.mockImplementation((url, config) => {
    if (url === '/api/token/' || url === '/api/token/admin/search') {
      const pending = deferred();
      requests.push({ ...pending, url, params: config.params });
      return pending.promise;
    }
    return Promise.resolve(response([]));
  });
});
async function search(name) {
  fireEvent.change(screen.getByPlaceholderText('搜索令牌的名称...'), { target: { value: name } });
  fireEvent.submit(screen.getByPlaceholderText('搜索令牌的名称...').closest('form'));
  await waitFor(() => expect(requests.at(-1).params.keyword).toBe(name));
}

describe('latest token list request wins', () => {
  it.each(['light', 'dark'])('loads and pages normally in %s mode', async (mode) => {
    render(<TokenList />, { wrapper: ({ children }) => <Wrapper mode={mode}>{children}</Wrapper> });
    expect(requests[0].params).toMatchObject({ page: 1, keyword: '', order: '-id' });
    await act(async () => requests[0].resolve(rows('Initial')));
    expect(screen.getByText('Initial')).toBeTruthy();
    expect(screen.queryByRole('progressbar')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Go to next page' }));
    await waitFor(() => expect(requests.at(-1).params.page).toBe(2));
    await act(async () => requests.at(-1).resolve(rows('Page 2')));
    expect(screen.getByText('Page 2')).toBeTruthy();
  });
  it('keeps new search results when the old request completes last', async () => {
    render(<TokenList />, { wrapper: Wrapper });
    await act(async () => requests[0].resolve(rows('Initial')));
    await search('old');
    const old = requests.at(-1);
    await search('new');
    const latest = requests.at(-1);
    await act(async () => latest.resolve(rows('New result')));
    expect(screen.getByText('New result')).toBeTruthy();
    await act(async () => old.resolve(rows('Old result')));
    expect(screen.queryByText('Old result')).toBeNull();
    expect(screen.getByText('New result')).toBeTruthy();
    expect(screen.getByPlaceholderText('搜索令牌的名称...').value).toBe('new');
  });

  it('does not clear loading or show stale errors while the latest page is pending', async () => {
    render(<TokenList />, { wrapper: Wrapper });
    const old = requests[0];
    await search('latest');
    const latest = requests.at(-1);
    await act(async () => old.resolve({ data: { success: false, message: 'obsolete error' } }));
    expect(showError).not.toHaveBeenCalled();
    expect(screen.getByRole('progressbar')).toBeTruthy();
    await act(async () => latest.resolve(rows('Latest')));
    expect(screen.queryByRole('progressbar')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Go to next page' }));
    await waitFor(() => expect(requests.at(-1).params.page).toBe(2));
    await act(async () => requests.at(-1).resolve(rows('Page 2')));
    expect(screen.getByText('Page 2')).toBeTruthy();
  });

  it('ignores a pending list response after unmount', async () => {
    const view = render(<TokenList />, { wrapper: Wrapper });
    const pending = requests[0];
    view.unmount();
    await act(async () => pending.resolve({ data: { success: false, message: 'obsolete error' } }));
    expect(showError).not.toHaveBeenCalled();
  });
  it('keeps loading while an obsolete success completes', async () => {
    render(<TokenList />, { wrapper: Wrapper });
    const old = requests[0];
    await search('latest');
    const latest = requests.at(-1);
    await act(async () => old.resolve(rows('Obsolete')));
    expect(screen.queryByText('Obsolete')).toBeNull();
    expect(screen.getByRole('progressbar')).toBeTruthy();
    await act(async () => latest.resolve(rows('Current')));
    expect(screen.getByText('Current')).toBeTruthy();
    expect(screen.queryByRole('progressbar')).toBeNull();
  });

  it('ignores rejected obsolete requests but handles a current rejection and recovery', async () => {
    const error = vi.spyOn(console, 'error').mockImplementation(() => {});
    try {
      render(<TokenList />, { wrapper: Wrapper });
      const old = requests[0];
      await search('latest');
      const latest = requests.at(-1);
      await act(async () => old.reject(new Error('Obsolete')));
      expect(error).not.toHaveBeenCalled();
      expect(screen.getByRole('progressbar')).toBeTruthy();
      const currentError = new Error('Current');
      await act(async () => latest.reject(currentError));
      expect(error).toHaveBeenCalledExactlyOnceWith(currentError);
      expect(screen.queryByRole('progressbar')).toBeNull();
      fireEvent.click(screen.getByRole('button', { name: '刷新' }));
      await waitFor(() => expect(requests).toHaveLength(3));
      await act(async () => requests.at(-1).resolve(rows('Recovered')));
      expect(screen.getByText('Recovered')).toBeTruthy();
    } finally {
      error.mockRestore();
    }
  });

  it('shows a current business error and stops loading', async () => {
    render(<TokenList />, { wrapper: Wrapper });
    await act(async () => requests[0].resolve({ data: { success: false, message: 'Current failure' } }));
    expect(showError).toHaveBeenCalledExactlyOnceWith('Current failure');
    expect(screen.queryByRole('progressbar')).toBeNull();
  });
  it('uses current admin filters and ignores the previous user results', async () => {
    store.dispatch({ type: LOGIN, payload: { role: 10 } });
    render(<TokenList />, { wrapper: Wrapper });
    const normal = requests[0];
    fireEvent.click(screen.getByText('管理员搜索'));
    fireEvent.change(screen.getByLabelText('用户ID'), { target: { value: '101' } });
    await waitFor(() => expect(requests.at(-1).params.user_id).toBe(101));
    const previous = requests.at(-1);
    expect(previous.url).toBe('/api/token/admin/search');
    fireEvent.change(screen.getByLabelText('用户ID'), { target: { value: '202' } });
    await waitFor(() => expect(requests.at(-1).params.user_id).toBe(202));
    const current = requests.at(-1);
    await act(async () => current.resolve(rows('Current admin result')));
    await act(async () => previous.resolve(rows('Previous user result')));
    await act(async () => normal.resolve(rows('Normal stale result')));
    expect(screen.getByText('Current admin result')).toBeTruthy();
    expect(screen.queryByText('Previous user result')).toBeNull();
    expect(screen.queryByText('Normal stale result')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '清除' }));
    await waitFor(() => expect(requests.at(-1).url).toBe('/api/token/'));
    expect(requests.at(-1).params.page).toBe(1);
    await act(async () => requests.at(-1).resolve(rows('Normal restored')));
    expect(screen.getByText('Normal restored')).toBeTruthy();
  });
});

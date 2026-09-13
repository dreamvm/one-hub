import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { Wrapper, response, deferred } from './ui-test-utils';
import { API } from '../src/utils/api';
import { showError } from '../src/utils/common';
import ChannelList from '../src/views/Channel';

vi.mock('../src/utils/api', () => ({ API: { get: vi.fn() } }));
vi.mock('../src/utils/common', async (original) => ({ ...(await original()), showError: vi.fn(), showSuccess: vi.fn() }));
vi.mock('@mui/material/useMediaQuery', () => ({ default: () => true }));
vi.mock('@iconify/react', () => ({ Icon: () => <span /> }));
// Real list, toolbar, pagination and effects; isolate unrelated row operations/editors.
vi.mock('../src/views/Channel/component/TableRow', () => ({
  default: ({ item }) => (
    <tr>
      <td>{item.name}</td>
    </tr>
  )
}));
vi.mock('../src/views/Channel/component/EditModal', () => ({ default: () => null }));
vi.mock('../src/views/Channel/component/BatchModal', () => ({ default: () => null }));

const rows = (name) => response({ data: [{ id: 1, name }], total_count: 30 });
let requests;
beforeEach(() => {
  vi.clearAllMocks();
  requests = [];
  API.get.mockImplementation((url, config) => {
    if (url === '/api/channel/') {
      const pending = deferred();
      requests.push({ ...pending, params: config.params });
      return pending.promise;
    }
    return Promise.resolve(response([]));
  });
});
async function search(name) {
  fireEvent.change(screen.getByPlaceholderText('渠道名称'), { target: { value: name } });
  fireEvent.click(screen.getByRole('button', { name: '搜索', exact: true }));
  await waitFor(() => expect(requests.at(-1).params.name).toBe(name));
}

describe('latest channel list request wins', () => {
  it('keeps new search results when the old request completes last', async () => {
    render(<ChannelList />, { wrapper: Wrapper });
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
    expect(screen.getByPlaceholderText('渠道名称').value).toBe('new');
  });

  it('does not clear loading or show stale errors while the latest page is pending', async () => {
    render(<ChannelList />, { wrapper: Wrapper });
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
    const view = render(<ChannelList />, { wrapper: Wrapper });
    const pending = requests[0];
    view.unmount();
    await act(async () => pending.resolve({ data: { success: false, message: 'obsolete error' } }));
    expect(showError).not.toHaveBeenCalled();
  });
});

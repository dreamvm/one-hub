import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { Wrapper, response } from './ui-test-utils';
import { store } from '../src/store';
import { LOGIN } from '../src/store/actions';
import { API } from '../src/utils/api';
import { showError } from '../src/utils/common';
import Token from '../src/views/Token';
import { UserContext } from '../src/contexts/UserContext';

vi.mock('../src/utils/api', () => ({ API: { get: vi.fn(), put: vi.fn(), delete: vi.fn() } }));
vi.mock('../src/utils/common', async (original) => ({ ...(await original()), showError: vi.fn(), showSuccess: vi.fn() }));
vi.mock('@iconify/react', () => ({ Icon: ({ icon }) => <span data-testid={icon} /> }));
vi.mock('../src/views/Token/component/EditModal', () => ({ default: () => null }));
const loadUserGroup = vi.fn();
const token = {
  id: 11,
  user_id: 2,
  name: 'Synthetic token',
  status: 1,
  expired_time: -1,
  remain_quota: 1000,
  used_quota: 0,
  group: '',
  key: 'synthetic'
};
let owner;
beforeEach(() => {
  vi.clearAllMocks();
  owner = 2;
  store.dispatch({ type: LOGIN, payload: { id: 1, role: 10 } });
  API.get.mockImplementation((url) =>
    Promise.resolve(response({ data: [{ ...token, user_id: url === '/api/token/' ? 1 : owner }], total_count: 1 }))
  );
  API.put.mockResolvedValue(response());
  API.delete.mockResolvedValue(response());
});
async function setup(admin = false, mode = 'light') {
  render(
    <UserContext.Provider value={{ loadUserGroup }}>
      <Token />
    </UserContext.Provider>,
    { wrapper: ({ children }) => <Wrapper mode={mode}>{children}</Wrapper> }
  );
  await screen.findByText('Synthetic token');
  if (admin) {
    fireEvent.click(screen.getByText('管理员搜索'));
    fireEvent.change(screen.getByLabelText('用户ID'), { target: { value: String(owner) } });
    await waitFor(() => expect(API.get).toHaveBeenCalledWith('/api/token/admin/search', expect.anything()));
    await waitFor(() => expect(screen.getByText(`${owner} - -`)).toBeTruthy());
  }
}
function menu() {
  fireEvent.click(screen.getByTestId('solar:menu-dots-circle-bold-duotone').closest('button'));
}
describe('token actions use existing ownership contracts', () => {
  it.each(['light', 'dark'])('updates another owner through the existing admin status endpoint in %s mode', async (mode) => {
    await setup(true, mode);
    fireEvent.click(screen.getByRole('checkbox'));
    await waitFor(() => expect(API.put).toHaveBeenCalledWith('/api/token/admin?status_only=true', { id: 11, status: 2 }));
    await waitFor(() => expect(screen.getByRole('checkbox').checked).toBe(false));
  });
  it('does not offer unsupported deletion for another owner but keeps edit', async () => {
    await setup(true);
    menu();
    expect(screen.getByRole('menuitem', { name: '编辑' })).toBeTruthy();
    expect(screen.queryByRole('menuitem', { name: '删除' })).toBeNull();
    expect(API.delete).not.toHaveBeenCalled();
  });
  it.each([false, true])('preserves owner deletion in admin search=%s', async (admin) => {
    owner = 1;
    await setup(admin);
    menu();
    fireEvent.click(screen.getByRole('menuitem', { name: '删除' }));
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: '删除' }));
    await waitFor(() => expect(API.delete).toHaveBeenCalledWith('/api/token/11'));
  });
  it('preserves the normal status endpoint when no admin filters are active', async () => {
    await setup();
    fireEvent.click(screen.getByText('管理员搜索'));
    fireEvent.click(screen.getByRole('checkbox'));
    await waitFor(() => expect(API.put).toHaveBeenCalledWith('/api/token/?status_only=true', { id: 11, status: 2 }));
  });
  it('uses the admin endpoint when searching only by token ID', async () => {
    await setup();
    fireEvent.click(screen.getByText('管理员搜索'));
    fireEvent.change(screen.getByLabelText('令牌ID'), { target: { value: '11' } });
    await waitFor(() => expect(screen.getByText('2 - -')).toBeTruthy());
    fireEvent.click(screen.getByRole('checkbox'));
    await waitFor(() => expect(API.put).toHaveBeenCalledWith('/api/token/admin?status_only=true', { id: 11, status: 2 }));
  });
  it('preserves status when the admin endpoint rejects the operation', async () => {
    API.put.mockResolvedValueOnce({ data: { success: false, message: 'Synthetic rejection' } });
    await setup(true);
    fireEvent.click(screen.getByRole('checkbox'));
    await waitFor(() => expect(showError).toHaveBeenCalledWith('Synthetic rejection'));
    expect(screen.getByRole('checkbox').checked).toBe(true);
  });
  it('keeps status unchanged after a network error and permits retry', async () => {
    API.put.mockRejectedValueOnce(new Error('Synthetic unavailable'));
    await setup();
    fireEvent.click(screen.getByRole('checkbox'));
    await waitFor(() => expect(showError).toHaveBeenCalled());
    expect(screen.getByRole('checkbox').checked).toBe(true);
    fireEvent.click(screen.getByRole('checkbox'));
    await waitFor(() => expect(screen.getByRole('checkbox').checked).toBe(false));
  });
});

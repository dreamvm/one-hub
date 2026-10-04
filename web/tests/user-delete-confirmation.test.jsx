import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, within, waitFor } from '@testing-library/react';
import { Wrapper } from './ui-test-utils';
import UsersTableRow from '../src/views/User/component/TableRow';

vi.mock('@iconify/react', () => ({ Icon: ({ icon }) => <span data-testid={icon} /> }));
const manageUser = vi.fn();
const item = {
  id: 3,
  username: 'synthcheck',
  display_name: 'Display only',
  role: 1,
  status: 1,
  quota: 0,
  used_quota: 0,
  request_count: 0,
  group: 'default'
};
beforeEach(() => {
  vi.clearAllMocks();
  manageUser.mockResolvedValue({ success: true });
});
function open(mode = 'light') {
  render(
    <table>
      <tbody>
        <UsersTableRow item={item} manageUser={manageUser} handleOpenModal={vi.fn()} setModalUserId={vi.fn()} />
      </tbody>
    </table>,
    { wrapper: ({ children }) => <Wrapper mode={mode}>{children}</Wrapper> }
  );
  fireEvent.click(within(screen.getByRole('row')).getByRole('button'));
  fireEvent.click(screen.getByRole('menuitem', { name: '删除', exact: true }));
}
describe('user deletion identifies the same account that will be submitted', () => {
  it.each(['light', 'dark'])('shows the selected username before confirmation in %s mode', (mode) => {
    open(mode);
    expect(screen.getByRole('dialog').textContent).toContain('synthcheck');
    expect(screen.getByRole('dialog').textContent).not.toContain('Display only');
    expect(manageUser).not.toHaveBeenCalled();
  });
  it('cancels without a request and submits the selected username only after confirmation', async () => {
    open();
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: '关闭' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(manageUser).not.toHaveBeenCalled();
    fireEvent.click(within(screen.getByRole('row')).getByRole('button'));
    fireEvent.click(screen.getByRole('menuitem', { name: '删除', exact: true }));
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: '删除', exact: true }));
    await waitFor(() => expect(manageUser).toHaveBeenCalledExactlyOnceWith('synthcheck', 'delete', ''));
  });
});

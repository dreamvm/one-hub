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
function open(mode = 'light', action = '删除') {
  render(
    <table>
      <tbody>
        <UsersTableRow item={item} manageUser={manageUser} handleOpenModal={vi.fn()} setModalUserId={vi.fn()} />
      </tbody>
    </table>,
    { wrapper: ({ children }) => <Wrapper mode={mode}>{children}</Wrapper> }
  );
  within(screen.getByRole('row')).getByRole('button').focus();
  fireEvent.click(within(screen.getByRole('row')).getByRole('button'));
  fireEvent.click(screen.getByRole('menuitem', { name: action, exact: true }));
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

// Closing must not leave restored focus in a hidden page during the exit animation.
describe('user dialog focus restoration', () => {
  it.each([
    ['light', '删除', '关闭'],
    ['dark', '删除', '关闭'],
    ['light', '增减额度', '取消'],
    ['dark', '增减额度', '取消']
  ])('restores visible page focus in %s mode after %s', async (mode, action, cancel) => {
    open(mode, action);
    await waitFor(() => expect(screen.queryByRole('menuitem', { name: '删除', exact: true })).toBeNull());
    const trigger = within(screen.getByRole('row', { hidden: true })).getByRole('button', { hidden: true });
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: cancel }));
    expect(document.activeElement).toBe(trigger);
    expect(trigger.closest('[aria-hidden="true"]')).toBeNull();
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(manageUser).not.toHaveBeenCalled();
  });
});

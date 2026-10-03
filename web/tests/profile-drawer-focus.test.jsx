import React, { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { createTheme, ThemeProvider } from '@mui/material/styles';
import { MemoryRouter } from 'react-router-dom';
import Profile from '../src/layout/MainLayout/Header/Profile';
import ProfileDrawer from '../src/layout/MainLayout/ProfileDrawer';

vi.mock('react-redux', () => ({
  useSelector: (selector) => selector({ account: { user: { username: 'synthetic-user' }, userGroup: {} } })
}));
vi.mock('react-i18next', () => ({ useTranslation: () => ({ t: (key) => key }) }));
vi.mock('../src/hooks/useLogin', () => ({ default: () => ({ logout: vi.fn() }) }));
vi.mock('../src/utils/common', () => ({ calculateQuota: (value) => value }));
vi.mock('@iconify/react', () => ({ Icon: () => null }));

function Harness({ mode }) {
  const [open, setOpen] = useState(false);
  return (
    <MemoryRouter>
      <ThemeProvider theme={createTheme({ palette: { mode } })}>
        <Profile toggleProfileDrawer={() => setOpen(true)} />
        <ProfileDrawer open={open} onClose={() => setOpen(false)} />
      </ThemeProvider>
    </MemoryRouter>
  );
}

describe('profile drawer focus', () => {
  it.each(['light', 'dark'])('provides a focusable named trigger and restores focus after closing in %s mode', async (mode) => {
    render(<Harness mode={mode} />);
    const trigger = screen.getByRole('button', { name: 'profile' });
    expect(trigger.tagName).toBe('BUTTON');
    expect(trigger.tabIndex).toBe(0);
    trigger.focus();
    fireEvent.click(trigger);
    const logout = await screen.findByRole('button', { name: 'menu.signout' });
    logout.focus();
    fireEvent.keyDown(logout, { key: 'Escape' });
    await waitFor(() => expect(document.activeElement).toBe(trigger));
    await waitFor(() => expect(screen.queryByRole('button', { name: 'menu.signout' })).toBeNull());
  });

  it('still opens the drawer through the existing click callback', () => {
    const open = vi.fn();
    const { container } = render(<ThemeProvider theme={createTheme()}><Profile toggleProfileDrawer={open} /></ThemeProvider>);
    fireEvent.click(container.querySelector('img'));
    expect(open).toHaveBeenCalledTimes(1);
  });
});

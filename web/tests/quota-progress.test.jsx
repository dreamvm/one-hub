import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { useSelector } from 'react-redux';
import MenuCard from '../src/layout/MainLayout/Sidebar/MenuCard';
import { Wrapper } from './ui-test-utils';

vi.mock('react-redux', async (importOriginal) => ({ ...(await importOriginal()), useSelector: vi.fn() }));

describe.each(['light', 'dark'])('quota progress in %s mode', (mode) => {
  beforeEach(() => {
    localStorage.removeItem('quota_per_unit');
  });

  it.each([
    ['empty account', undefined, 0],
    ['zero quota', { quota: 0, used_quota: 0 }, 0],
    ['unused balance', { quota: 100, used_quota: 0 }, 0],
    ['normal usage', { quota: 75, used_quota: 25 }, 25],
    ['sub-cent balances', { quota: 1, used_quota: 1 }, 50],
    ['exhausted balance', { quota: 0, used_quota: 100 }, 100],
    ['negative balance', { quota: -10, used_quota: 100 }, 100],
    ['invalid values', { quota: 'invalid', used_quota: Infinity }, 0]
  ])('renders a finite bounded percentage for %s', (_name, user, expected) => {
    useSelector.mockImplementation((select) => select({ account: { user } }));
    const { container } = render(
      <Wrapper mode={mode}>
        <MenuCard />
      </Wrapper>
    );
    expect(screen.getByRole('progressbar').getAttribute('aria-valuenow')).toBe(String(expected));
    expect(container.textContent).not.toMatch(/NaN|Infinity/);
    expect(container.textContent).toContain(`(${expected}%)`);
  });

  it.each(['0', '-1', 'invalid'])('handles invalid quota conversion %s', (unit) => {
    localStorage.setItem('quota_per_unit', unit);
    useSelector.mockReturnValue({ user: { quota: 500000, used_quota: 500000 } });
    const { container } = render(
      <Wrapper mode={mode}>
        <MenuCard />
      </Wrapper>
    );
    expect(container.textContent).toContain('$1.00 (50%)');
    expect(screen.getByRole('progressbar').getAttribute('aria-valuenow')).toBe('50');
  });

  it('clears the previous account balance when user state is cleared', () => {
    useSelector.mockReturnValue({ user: { quota: 500000, used_quota: 500000 } });
    const { rerender, container } = render(
      <Wrapper mode={mode}>
        <MenuCard />
      </Wrapper>
    );
    useSelector.mockReturnValue({ user: null });
    rerender(
      <Wrapper mode={mode}>
        <MenuCard />
      </Wrapper>
    );
    expect(container.textContent).toContain('$0.00 (0%)');
    expect(screen.getByRole('progressbar').getAttribute('aria-valuenow')).toBe('0');
  });
});

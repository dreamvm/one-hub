import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { act, render, screen, waitFor } from '@testing-library/react';
import Home from '../src/views/Home';
import { API } from '../src/utils/api';
import { showError } from '../src/utils/common';

const translation = vi.hoisted(() => ({ language: 'en' }));

vi.mock('../src/utils/api', () => ({ API: { get: vi.fn() } }));
vi.mock('../src/utils/common', () => ({ showError: vi.fn() }));
vi.mock('react-i18next', () => ({ useTranslation: () => ({ t: (key) => `${translation.language}:${key}` }) }));

beforeEach(() => {
  localStorage.clear();
  translation.language = 'en';
  API.get.mockReset();
  showError.mockClear();
});

describe('home loading lifecycle', () => {
  it('renders successful configured content', async () => {
    API.get.mockResolvedValue({ data: { success: true, data: '# Synthetic homepage' } });
    render(<Home />);
    expect(await screen.findByRole('heading', { name: 'Synthetic homepage' })).toBeTruthy();
    expect(screen.queryByRole('progressbar')).toBeNull();
  });

  it('renders the default home when configuration is empty', async () => {
    API.get.mockResolvedValue({ data: { success: true, data: '' } });
    render(<Home />);
    expect(await screen.findByText('One Hub')).toBeTruthy();
    expect(screen.queryByRole('progressbar')).toBeNull();
  });

  it('ends loading and shows an error after a network failure', async () => {
    API.get.mockRejectedValue(new Error('synthetic network failure'));
    render(<Home />);
    await waitFor(() => expect(screen.queryByRole('progressbar')).toBeNull());
    expect(screen.getByText('en:home.loadingErr')).toBeTruthy();
  });
  it('ends loading when the shared interceptor resolves with no response', async () => {
    API.get.mockResolvedValue(undefined);
    render(<Home />);
    expect(await screen.findByText('en:home.loadingErr')).toBeTruthy();
    expect(screen.queryByRole('progressbar')).toBeNull();
    expect(screen.queryByText('One Hub')).toBeNull();
  });

  it('shows an application error and updates its language without another request', async () => {
    API.get.mockResolvedValue({ data: { success: false, message: 'Synthetic unavailable' } });
    const { rerender } = render(<Home />);
    expect(await screen.findByText('en:home.loadingErr')).toBeTruthy();
    expect(showError).toHaveBeenCalledWith('Synthetic unavailable');
    translation.language = 'zh';
    rerender(<Home />);
    expect(screen.getByText('zh:home.loadingErr')).toBeTruthy();
    expect(API.get).toHaveBeenCalledTimes(1);
  });

  it('does not interpret successful content as error state', async () => {
    API.get.mockResolvedValue({ data: { success: true, data: 'en:home.loadingErr' } });
    const { rerender } = render(<Home />);
    const content = await screen.findByText('en:home.loadingErr');
    expect(content.closest('.content-viewer')).not.toBeNull();
    translation.language = 'zh';
    rerender(<Home />);
    expect(screen.getByText('en:home.loadingErr').closest('.content-viewer')).not.toBeNull();
    expect(screen.queryByText('zh:home.loadingErr')).toBeNull();
    expect(API.get).toHaveBeenCalledTimes(1);
  });

  it('ignores an obsolete request after StrictMode effect cleanup', async () => {
    let resolveOld;
    API.get.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveOld = resolve;
        })
    );
    API.get.mockResolvedValueOnce({ data: { success: true, data: '# Current content' } });
    render(
      <React.StrictMode>
        <Home />
      </React.StrictMode>
    );
    expect(await screen.findByRole('heading', { name: 'Current content' })).toBeTruthy();
    await act(async () => {
      resolveOld({ data: { success: true, data: '# Obsolete content' } });
    });
    expect(screen.queryByText('Obsolete content')).toBeNull();
    expect(localStorage.getItem('home_page_content')).toBe('# Current content');
  });

  it('does not notify or persist a result after unmount', async () => {
    let resolveRequest;
    API.get.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveRequest = resolve;
        })
    );
    const { unmount } = render(<Home />);
    unmount();
    await act(async () => {
      resolveRequest({ data: { success: false, message: 'Obsolete failure' } });
    });
    expect(showError).not.toHaveBeenCalled();
    expect(localStorage.getItem('home_page_content')).toBeNull();
  });
});

import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { createTheme, ThemeProvider } from '@mui/material/styles';
import { progressEventReducer } from '../node_modules/axios/lib/helpers/progressEventReducer.js';
import { API } from '../src/utils/api';
import { showError } from '../src/utils/common';
import { ChannelCheck } from '../src/views/Channel/component/ChannelCheck';

vi.mock('../src/utils/api', () => ({ API: { post: vi.fn() } }));
vi.mock('../src/utils/common', () => ({ showError: vi.fn() }));
vi.mock('@iconify/react', () => ({ Icon: () => null }));

const result = (model = 'gpt-synthetic') => `data:${JSON.stringify({ type: 'result', data: { model, process: [] } })}\n\n`;
const open = (mode = 'light') => {
  render(
    <ThemeProvider theme={createTheme({ palette: { mode } })}>
      <ChannelCheck item={{ id: 7, models: 'gpt-synthetic' }} open onClose={vi.fn()} />
    </ThemeProvider>
  );
  fireEvent.click(screen.getByRole('button', { name: '开始检测' }));
};

beforeEach(() => vi.clearAllMocks());

describe('channel check Axios text progress', () => {
  it.each(['light', 'dark'])('shows wrapped progress before completion in %s mode', async (mode) => {
    let finish;
    API.post.mockImplementation(async (_url, _body, config) => {
      const [notify, flush] = progressEventReducer(config.onDownloadProgress, true);
      const xhr = { response: result() };
      // Axios wraps the native event; throttled delivery may outlive currentTarget.
      notify({ loaded: xhr.response.length, lengthComputable: false, target: xhr, currentTarget: null });
      flush();
      return new Promise((resolve) => {
        finish = resolve;
      });
    });
    open(mode);
    expect(await screen.findByText('检测通过')).toBeTruthy();
    expect(screen.getByRole('button', { name: '开始检测' }).disabled).toBe(true);
    expect(API.post.mock.calls[0].slice(0, 2)).toEqual(['/api/sse/channel/check', { id: 7, models: 'gpt-synthetic' }]);
    await act(async () => finish({ status: 200, data: result() }));
    expect(screen.getAllByText('检测通过')).toHaveLength(1);
    expect(showError).not.toHaveBeenCalled();
  });

  it('uses completed text when no progress callback is delivered', async () => {
    API.post.mockResolvedValue({ status: 200, data: result() });
    open();
    expect(await screen.findByText('检测通过')).toBeTruthy();
    await waitFor(() => expect(screen.getByRole('button', { name: '开始检测' }).disabled).toBe(false));
  });

  it('handles partial cumulative text, repeated model updates and a targetless notification', async () => {
    API.post.mockImplementation(async (_url, _body, config) => {
      config.onDownloadProgress({ loaded: 0 });
      config.onDownloadProgress({ event: { target: { response: result().slice(0, 20) } } });
      config.onDownloadProgress({ event: { target: { response: result() + result('gpt-second') } } });
      return { status: 200, data: result() + result('gpt-second') };
    });
    open();
    await waitFor(() => expect(screen.getAllByText('检测通过')).toHaveLength(2));
    expect(showError).not.toHaveBeenCalled();
  });

  it('reports a failed HTTP response and allows retry', async () => {
    API.post.mockResolvedValue({ status: 500, data: { message: '合成检测失败' } });
    open();
    await waitFor(() => expect(showError).toHaveBeenCalledWith('合成检测失败'));
    expect(screen.getByRole('button', { name: '开始检测' }).disabled).toBe(false);
  });

  it('preserves rejected request reporting and allows retry', async () => {
    API.post.mockRejectedValue(new Error('合成连接失败'));
    open();
    await waitFor(() => expect(showError).toHaveBeenCalledWith('合成连接失败'));
    expect(screen.getByRole('button', { name: '开始检测' }).disabled).toBe(false);
    expect(screen.queryByText('检测通过')).toBeNull();
  });
});

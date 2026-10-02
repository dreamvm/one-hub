import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { Wrapper, response, zh } from './ui-test-utils';
import { API } from '../src/utils/api';
import { showError } from '../src/utils/common';
import ChannelList from '../src/views/Channel';

vi.mock('../src/utils/api', () => ({ API: { get: vi.fn() } }));
vi.mock('../src/utils/common', async (original) => ({ ...(await original()), showError: vi.fn(), showSuccess: vi.fn() }));
vi.mock('@mui/material/useMediaQuery', () => ({ default: () => true }));
vi.mock('@iconify/react', () => ({ Icon: () => <span /> }));
vi.mock('../src/views/Channel/component/TableRow', () => ({ default: () => null }));
vi.mock('../src/views/Channel/component/BatchModal', () => ({ default: () => null }));
// Observe the choices passed to a newly opened editor without loading Monaco.
vi.mock('../src/views/Channel/component/EditModal', () => ({
  default: ({ open, modelOptions }) => (open ? <output aria-label="Model choices">{JSON.stringify(modelOptions)}</output> : null)
}));

const modelResponse = (data) => ({ data: { object: 'list', data } });
let modelReply;
beforeEach(() => {
  vi.clearAllMocks();
  modelReply = modelResponse([]);
  API.get.mockImplementation((url) => {
    if (url === '/api/channel/models') return modelReply instanceof Error ? Promise.reject(modelReply) : Promise.resolve(modelReply);
    return Promise.resolve(response(url === '/api/channel/' ? { data: [], total_count: 0 } : []));
  });
});

async function openEditor() {
  await act(async () => render(<ChannelList />, { wrapper: Wrapper }));
  fireEvent.click(screen.getByRole('button', { name: zh.channel_index.newChannel, exact: true }));
  return JSON.parse(screen.getByLabelText('Model choices').textContent);
}

describe('channel model choices', () => {
  it.each([null, []])('accepts an empty successful model collection: %j', async (data) => {
    modelReply = modelResponse(data);
    expect(await openEditor()).toEqual([]);
    expect(showError).not.toHaveBeenCalled();
  });

  it('keeps supplier and model ordering for populated results', async () => {
    modelReply = modelResponse([
      { id: 'z', owned_by: 'Beta' },
      { id: 'b', owned_by: 'Alpha' },
      { id: 'a', owned_by: 'Alpha' }
    ]);
    expect(await openEditor()).toEqual([
      { id: 'a', group: 'Alpha' },
      { id: 'b', group: 'Alpha' },
      { id: 'z', group: 'Beta' }
    ]);
    expect(showError).not.toHaveBeenCalled();
  });

  it('reports a business failure instead of treating it as an empty success', async () => {
    modelReply = { data: { success: false, message: 'Synthetic model read failure', data: null } };
    expect(await openEditor()).toEqual([]);
    expect(showError).toHaveBeenCalledExactlyOnceWith('Synthetic model read failure');
  });

  it('preserves network failure reporting', async () => {
    modelReply = new Error('Synthetic network unavailable');
    expect(await openEditor()).toEqual([]);
    expect(showError).toHaveBeenCalledExactlyOnceWith('Synthetic network unavailable');
  });
});

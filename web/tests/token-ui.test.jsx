import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { Wrapper, response, deferred, zh } from './ui-test-utils';
import { API } from '../src/utils/api';
import EditModal from '../src/views/Token/component/EditModal';

vi.mock('../src/utils/api', () => ({ API: { get: vi.fn(), put: vi.fn(), post: vi.fn() } }));
vi.mock('../src/utils/common', async (original) => ({ ...(await original()), showError: vi.fn(), showSuccess: vi.fn() }));

const token = (id = 1, extra = {}) => ({ id, name: `Token ${id}`, user_id: 10, group: '', ...extra });
const props = () => ({ open: true, tokenId: 1, onCancel: vi.fn(), onOk: vi.fn(), userGroupOptions: [] });
const nameField = () => screen.getByRole('textbox', { name: '名称' });
const submit = () => screen.getByRole('button', { name: '提交' });
let read;
beforeEach(() => {
  vi.clearAllMocks();
  read = vi.fn().mockResolvedValue(response(token()));
  API.get.mockImplementation((url, config) => {
    if (url === '/api/model_ownedby/') return Promise.resolve(response([]));
    if (url === '/api/available_model') return Promise.resolve(response({}));
    return read(url, config);
  });
  API.put.mockResolvedValue(response());
  API.post.mockResolvedValue(response());
});

describe('token edit session protection', () => {
  it('blocks edits and submit until the exact token is loaded with a bounded timeout', async () => {
    const pending = deferred();
    read.mockReturnValue(pending.promise);
    render(<EditModal {...props()} />, { wrapper: Wrapper });
    expect(submit().disabled).toBe(true);
    expect(screen.queryByRole('textbox', { name: '名称' })).toBeNull();
    expect(read).toHaveBeenCalledWith('/api/token/1', { timeout: 30000 });
    fireEvent.click(submit());
    expect(API.post).not.toHaveBeenCalled();
    await act(async () => pending.resolve(response(token())));
    expect(nameField().value).toBe('Token 1');
  });

  it.each(['logical', 'network', 'missing', 'wrong-id', 'empty-response'])(
    'fails closed on %s read, then retries and updates without creating',
    async (failure) => {
      if (failure === 'network') read.mockRejectedValueOnce(new Error('synthetic network error'));
      else
        read.mockResolvedValueOnce(
          {
            logical: { data: { success: false } },
            missing: response(null),
            'wrong-id': response(token(2)),
            'empty-response': undefined
          }[failure]
        );
      read.mockResolvedValueOnce(response(token()));
      render(<EditModal {...props()} />, { wrapper: Wrapper });
      await screen.findByText(zh.token_index.loadFailed);
      expect(submit().disabled).toBe(true);
      expect(screen.queryByRole('textbox', { name: '名称' })).toBeNull();
      fireEvent.click(submit());
      expect(API.post).not.toHaveBeenCalled();
      fireEvent.click(screen.getByRole('button', { name: zh.token_index.retryLoad }));
      await waitFor(() => expect(nameField().value).toBe('Token 1'));
      fireEvent.click(submit());
      await waitFor(() => expect(API.put).toHaveBeenCalledTimes(1));
      expect(API.put.mock.calls[0][0]).toBe('/api/token/');
      expect(API.put.mock.calls[0][1]).toMatchObject({ id: 1, is_edit: true });
      expect(API.post).not.toHaveBeenCalled();
    }
  );

  it('ignores a late A read, preserves edited B on option refresh and saves only B', async () => {
    const pending = deferred();
    read.mockReturnValueOnce(pending.promise).mockResolvedValueOnce(response(token(2)));
    const initial = props();
    const view = render(<EditModal {...initial} />, { wrapper: Wrapper });
    view.rerender(<EditModal {...initial} tokenId={2} />);
    await waitFor(() => expect(nameField().value).toBe('Token 2'));
    fireEvent.change(nameField(), { target: { value: 'B edited' } });
    await act(async () => pending.resolve(response(token())));
    view.rerender(<EditModal {...initial} tokenId={2} userGroupOptions={[{ value: 'test', label: 'Test' }]} />);
    expect(nameField().value).toBe('B edited');
    fireEvent.click(submit());
    await waitFor(() => expect(API.put).toHaveBeenCalledTimes(1));
    expect(API.put.mock.calls[0][1]).toMatchObject({ id: 2, name: 'B edited' });
    expect(read).toHaveBeenCalledTimes(2);
  });

  it('reloads the same token after close/reopen, and does not reuse defaults or edits for create', async () => {
    read.mockResolvedValueOnce(response(token())).mockResolvedValueOnce(response(token(1, { name: 'Refreshed' })));
    const initial = props();
    const view = render(<EditModal {...initial} />, { wrapper: Wrapper });
    await waitFor(() => expect(nameField().value).toBe('Token 1'));
    view.rerender(<EditModal {...initial} open={false} />);
    view.rerender(<EditModal {...initial} />);
    await waitFor(() => expect(nameField().value).toBe('Refreshed'));
    view.rerender(<EditModal {...initial} tokenId={0} />);
    await waitFor(() => expect(nameField().value).toBe(''));
    fireEvent.change(nameField(), { target: { value: 'New token' } });
    fireEvent.click(submit());
    await waitFor(() => expect(API.post).toHaveBeenCalledTimes(1));
    expect(API.post.mock.calls[0][1]).toMatchObject({ is_edit: false, name: 'New token' });
    expect(API.post.mock.calls[0][1]).not.toHaveProperty('id');
    expect(API.put).not.toHaveBeenCalled();
    expect(read).toHaveBeenCalledTimes(2);
  });

  it('uses the administrator read/update endpoints and preserves token ownership', async () => {
    read.mockResolvedValueOnce(response({ data: [token()] }));
    render(<EditModal {...props()} adminMode />, { wrapper: Wrapper });
    await waitFor(() => expect(nameField().value).toBe('Token 1'));
    expect(read).toHaveBeenCalledWith('/api/token/admin/search', { params: { token_id: 1, page: 1, size: 1 }, timeout: 30000 });
    fireEvent.click(submit());
    await waitFor(() => expect(API.put).toHaveBeenCalledTimes(1));
    expect(API.put.mock.calls[0]).toEqual(['/api/token/admin', expect.objectContaining({ id: 1, user_id: 10, is_edit: true })]);
    expect(API.post).not.toHaveBeenCalled();
  });

  it('blocks an empty administrator search result', async () => {
    read.mockResolvedValueOnce(response({ data: [] }));
    render(<EditModal {...props()} adminMode />, { wrapper: Wrapper });
    await screen.findByText(zh.token_index.loadFailed);
    expect(submit().disabled).toBe(true);
    expect(API.post).not.toHaveBeenCalled();
  });

  it('preserves existing settings without mutating the read response or shared defaults', async () => {
    const original = Object.freeze(
      token(1, {
        setting: Object.freeze({
          billing_tag: 'billing-test',
          heartbeat: Object.freeze({ enabled: false, timeout_seconds: 45 }),
          limits: Object.freeze({
            custom_limit: { enabled: true },
            limits_ip_setting: Object.freeze({ enabled: true, whitelist: Object.freeze(['  ', '127.0.0.1']) })
          })
        })
      })
    );
    read.mockResolvedValueOnce(response(original));
    render(<EditModal {...props()} />, { wrapper: Wrapper });
    await waitFor(() => expect(nameField().value).toBe('Token 1'));
    fireEvent.click(submit());
    await waitFor(() => expect(API.put).toHaveBeenCalledTimes(1));
    expect(API.put.mock.calls[0][1].setting).toMatchObject({
      billing_tag: 'billing-test',
      heartbeat: { enabled: false, timeout_seconds: 45 },
      limits: { custom_limit: { enabled: true }, limits_ip_setting: { enabled: true, whitelist: ['127.0.0.1'] } }
    });
    expect(original).not.toHaveProperty('is_edit');
    expect(original.setting.limits.limits_ip_setting.whitelist).toEqual(['  ', '127.0.0.1']);
  });

  it.each(['network', 'logical'])('unlocks submit after %s save failure without losing the edits', async (failure) => {
    if (failure === 'network') API.put.mockRejectedValueOnce(new Error('synthetic save failure'));
    else API.put.mockResolvedValueOnce({ data: { success: false, message: 'synthetic rejection' } });
    const initial = props();
    render(<EditModal {...initial} />, { wrapper: Wrapper });
    await waitFor(() => expect(nameField().value).toBe('Token 1'));
    fireEvent.change(nameField(), { target: { value: 'Retry me' } });
    fireEvent.click(submit());
    await waitFor(() => expect(API.put).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(submit().disabled).toBe(false));
    expect(nameField().value).toBe('Retry me');
    expect(initial.onOk).not.toHaveBeenCalled();
  });

  it('ignores completion of a previous save after switching to another edit session', async () => {
    const pending = deferred();
    API.put.mockReturnValue(pending.promise);
    read.mockResolvedValueOnce(response(token())).mockResolvedValueOnce(response(token(2)));
    const initial = props();
    const view = render(<EditModal {...initial} />, { wrapper: Wrapper });
    await waitFor(() => expect(nameField().value).toBe('Token 1'));
    fireEvent.click(submit());
    await waitFor(() => expect(API.put).toHaveBeenCalledTimes(1));
    view.rerender(<EditModal {...initial} tokenId={2} />);
    await waitFor(() => expect(nameField().value).toBe('Token 2'));
    await act(async () => pending.resolve(response()));
    expect(initial.onOk).not.toHaveBeenCalled();
    expect(nameField().value).toBe('Token 2');
  });
});

import { beforeEach, describe, expect, it, vi } from 'vitest';
import axios from 'axios';
import { API, LoginCheckAPI } from '../src/utils/api';
import { showError } from '../src/utils/common';
import { store } from '../src/store';
import { LOGIN } from '../src/store/actions';

vi.mock('../src/utils/common', () => ({ showError: vi.fn() }));
vi.mock('../src/store', () => ({ store: { dispatch: vi.fn() } }));

const reply = (config, data = '{"success":true}', status = 200) => ({
  config, data, status, statusText: 'synthetic', headers: {}, request: {}
});

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
});

describe('real Axios through the application API factories', () => {
  it('preserves the configured base URL and JSON response on both instances', async () => {
    for (const api of [API, LoginCheckAPI]) {
      const adapter = vi.fn(async (config) => reply(config));
      const result = await api.get('/api/user/self', { adapter });
      expect(adapter.mock.calls[0][0].baseURL).toBe(import.meta.env.VITE_APP_SERVER || '/');
      expect(result.data).toEqual({ success: true });
    }
    expect(showError).not.toHaveBeenCalled();
  });

  it('clears the session and dispatches logout on an API 401', async () => {
    localStorage.setItem('user', JSON.stringify({ id: 1, username: 'synthetic' }));
    const adapter = async (config) => {
      throw new axios.AxiosError('Unauthorized', 'ERR_BAD_REQUEST', config, {}, reply(config, { message: '请重新登录' }, 401));
    };
    // Preserve the current interceptor contract: reports and consumes the error.
    expect(await API.get('/api/user/self', { adapter })).toBeUndefined();
    expect(localStorage.getItem('user')).toBeNull();
    expect(store.dispatch).toHaveBeenCalledTimes(1);
    expect(store.dispatch).toHaveBeenCalledWith({ type: LOGIN, payload: null });
    expect(showError).toHaveBeenCalledOnce();
    expect(showError.mock.calls[0][0].message).toBe('请重新登录');
  });

  it('keeps session state on an ordinary API error and reports its message', async () => {
    localStorage.setItem('user', 'synthetic');
    const adapter = async (config) => {
      throw new axios.AxiosError('Bad request', 'ERR_BAD_REQUEST', config, {}, reply(config, { message: '合成失败' }, 400));
    };
    expect(await API.post('/api/channel/', { name: '测试' }, { adapter })).toBeUndefined();
    expect(localStorage.getItem('user')).toBe('synthetic');
    expect(store.dispatch).not.toHaveBeenCalled();
    expect(showError.mock.calls[0][0].message).toBe('合成失败');
  });

  it('leaves LoginCheckAPI rejection observable without clearing API session state', async () => {
    localStorage.setItem('user', 'synthetic');
    const adapter = async (config) => {
      throw new axios.AxiosError('Unauthorized', 'ERR_BAD_REQUEST', config, {}, reply(config, {}, 401));
    };
    await expect(LoginCheckAPI.get('/api/user/self', { adapter })).rejects.toMatchObject({ response: { status: 401 } });
    expect(localStorage.getItem('user')).toBe('synthetic');
    expect(store.dispatch).not.toHaveBeenCalled();
    expect(showError).not.toHaveBeenCalled();
  });
});

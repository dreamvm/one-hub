import { beforeEach, describe, expect, it, vi } from 'vitest';
import { act, renderHook, waitFor } from '@testing-library/react';
import useLogin from '../src/hooks/useLogin';
import { API } from '../src/utils/api';
import { store } from '../src/store';
import { SET_USER_GROUP } from '../src/store/actions';
import { Wrapper } from './ui-test-utils';

vi.mock('../src/utils/api', () => ({ API: { get: vi.fn() }, LoginCheckAPI: { get: vi.fn() } }));
beforeEach(() => {
  API.get.mockReset();
  store.dispatch({ type: SET_USER_GROUP, payload: {} });
});
describe('user group loading request completion', () => {
  it('loads a successful group map without another request on rerender', async () => {
    const groups = { default: { name: 'Synthetic default', symbol: 'default', ratio: 1 } };
    API.get.mockResolvedValue({ data: { success: true, data: groups } });
    const { result, rerender } = renderHook(useLogin, { wrapper: Wrapper });
    act(() => {
      result.current.loadUserGroup();
    });
    await waitFor(() => expect(store.getState().account.userGroup).toEqual(groups));
    const callback = result.current.loadUserGroup;
    rerender();
    expect(result.current.loadUserGroup).toBe(callback);
    expect(API.get).toHaveBeenCalledTimes(1);
  });
  it('handles an asynchronous request rejection without changing group state', async () => {
    const failure = new Error('Synthetic unavailable');
    const errorLog = vi.spyOn(console, 'error').mockImplementation(() => {});
    API.get.mockRejectedValue(failure);
    const { result } = renderHook(useLogin, { wrapper: Wrapper });
    act(() => {
      result.current.loadUserGroup();
    });
    await waitFor(() => expect(errorLog).toHaveBeenCalledWith(failure));
    expect(store.getState().account.userGroup).toEqual({});
  });
  it('handles an undefined response from the shared interceptor', async () => {
    const errorLog = vi.spyOn(console, 'error').mockImplementation(() => {});
    API.get.mockResolvedValue(undefined);
    const { result } = renderHook(useLogin, { wrapper: Wrapper });
    act(() => {
      expect(result.current.loadUserGroup()).toEqual([]);
    });
    await waitFor(() => expect(errorLog).toHaveBeenCalledWith(expect.any(TypeError)));
    expect(store.getState().account.userGroup).toEqual({});
  });
  it('preserves existing groups on business failure and accepts a later recovery', async () => {
    const previous = { previous: { name: 'Previous', symbol: 'previous', ratio: 1 } };
    const recovered = { recovered: { name: 'Recovered', symbol: 'recovered', ratio: 2 } };
    store.dispatch({ type: SET_USER_GROUP, payload: previous });
    API.get
      .mockResolvedValueOnce({ data: { success: false, message: 'Synthetic business failure' } })
      .mockResolvedValueOnce({ data: { success: true, data: recovered } });
    const { result } = renderHook(useLogin, { wrapper: Wrapper });
    await act(async () => {
      result.current.loadUserGroup();
    });
    expect(store.getState().account.userGroup).toEqual(previous);
    await act(async () => {
      result.current.loadUserGroup();
    });
    expect(store.getState().account.userGroup).toEqual(recovered);
    expect(API.get).toHaveBeenCalledTimes(2);
  });
  it('still catches a synchronous API exception', () => {
    const failure = new Error('Synthetic synchronous failure');
    const errorLog = vi.spyOn(console, 'error').mockImplementation(() => {});
    API.get.mockImplementation(() => {
      throw failure;
    });
    const { result } = renderHook(useLogin, { wrapper: Wrapper });
    act(() => {
      expect(result.current.loadUserGroup()).toEqual([]);
    });
    expect(errorLog).toHaveBeenCalledWith(failure);
  });
});

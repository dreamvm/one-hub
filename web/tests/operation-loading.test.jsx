import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { Wrapper, response, deferred, zh } from './ui-test-utils';
import OperationSetting from '../src/views/Setting/component/OperationSetting';
import { LoadStatusContext } from '../src/contexts/StatusContext';
import { API } from '../src/utils/api';
import { showError } from '../src/utils/common';
vi.mock('../src/utils/api', () => ({ API: { get: vi.fn(), put: vi.fn() } }));
vi.mock('../src/utils/common', async (original) => ({ ...(await original()), showError: vi.fn(), showSuccess: vi.fn() }));
vi.mock('@iconify/react', () => ({ Icon: () => <span /> }));
vi.mock('../src/views/Setting/component/ChatLinksDataGrid', () => ({ default: () => null }));
const labels = zh.setting_index.operationSettings.generalSettings;
const loadStatus = vi.fn();
beforeEach(() => {
  vi.clearAllMocks();
  API.get.mockImplementation(async (url) =>
    response(url === '/api/option/safe_tools' ? [] : [{ key: 'TopUpLink', value: 'https://example.invalid/normal' }])
  );
});
function mount(mode = 'light') {
  return render(
    <LoadStatusContext.Provider value={loadStatus}>
      <OperationSetting />
    </LoadStatusContext.Provider>,
    {
      wrapper: ({ children }) => <Wrapper mode={mode}>{children}</Wrapper>
    }
  );
}
describe('operation settings initial read lifecycle', () => {
  it.each(['light', 'dark'])('loads once and does not refetch while editing in %s', async (mode) => {
    mount(mode);
    await waitFor(() => expect(screen.getByLabelText(labels.topUpLink.label).value).toBe('https://example.invalid/normal'));
    fireEvent.change(screen.getByLabelText(labels.topUpLink.label), { target: { value: 'https://example.invalid/edit' } });
    await act(async () => {});
    expect(API.get.mock.calls.map(([url]) => url)).toEqual(['/api/option/safe_tools', '/api/option/']);
    expect(screen.getByLabelText(labels.topUpLink.label).value).toBe('https://example.invalid/edit');
  });
  it('does not start the next initial read or show an obsolete error after unmount', async () => {
    const pending = deferred();
    API.get.mockReturnValue(pending.promise);
    const view = mount();
    view.unmount();
    await act(async () => pending.resolve({ data: { success: false, message: 'Synthetic old tools failure' } }));
    expect(showError).not.toHaveBeenCalled();
    expect(API.get).toHaveBeenCalledTimes(1);
  });
  it('ignores an obsolete option failure after unmount', async () => {
    const pending = deferred();
    API.get.mockImplementation((url) => (url === '/api/option/' ? pending.promise : Promise.resolve(response([]))));
    const view = mount();
    await waitFor(() => expect(API.get).toHaveBeenCalledTimes(2));
    view.unmount();
    await act(async () => pending.resolve({ data: { success: false, message: 'Synthetic old options failure' } }));
    expect(showError).not.toHaveBeenCalled();
  });
  it('preserves an edited value absent from a delayed response', async () => {
    const pending = deferred();
    API.get.mockImplementation((url) => (url === '/api/option/' ? pending.promise : Promise.resolve(response([]))));
    mount();
    await waitFor(() => expect(API.get).toHaveBeenCalledTimes(2));
    fireEvent.change(screen.getByLabelText(labels.topUpLink.label), { target: { value: 'https://example.invalid/unsaved' } });
    await act(async () => pending.resolve(response([{ key: 'QuotaPerUnit', value: 500000 }])));
    expect(screen.getByLabelText(labels.topUpLink.label).value).toBe('https://example.invalid/unsaved');
  });
  it('retains current failure feedback and still loads options after a tool rejection', async () => {
    const errorLog = vi.spyOn(console, 'error').mockImplementation(() => {});
    API.get.mockImplementation((url) =>
      url === '/api/option/safe_tools'
        ? Promise.reject(new Error('Synthetic unavailable'))
        : Promise.resolve(response([{ key: 'TopUpLink', value: 'https://example.invalid/recovered' }]))
    );
    mount();
    await waitFor(() => expect(screen.getByLabelText(labels.topUpLink.label).value).toBe('https://example.invalid/recovered'));
    expect(showError).toHaveBeenCalledExactlyOnceWith('获取安全工具列表失败');
    expect(errorLog).toHaveBeenCalledTimes(1);
  });
  it('does not log a rejected obsolete tool request', async () => {
    const errorLog = vi.spyOn(console, 'error').mockImplementation(() => {});
    const pending = deferred();
    API.get.mockReturnValue(pending.promise);
    const view = mount();
    view.unmount();
    await act(async () => pending.reject(new Error('Synthetic obsolete')));
    expect(errorLog).not.toHaveBeenCalled();
    expect(showError).not.toHaveBeenCalled();
    expect(API.get).toHaveBeenCalledTimes(1);
  });
  it('isolates the first StrictMode initialization from its replacement', async () => {
    const first = deferred();
    let tools = 0;
    API.get.mockImplementation((url) => {
      if (url === '/api/option/safe_tools' && ++tools === 1) return first.promise;
      return Promise.resolve(
        response(url === '/api/option/safe_tools' ? [] : [{ key: 'TopUpLink', value: 'https://example.invalid/current' }])
      );
    });
    render(
      <React.StrictMode>
        <LoadStatusContext.Provider value={loadStatus}>
          <OperationSetting />
        </LoadStatusContext.Provider>
      </React.StrictMode>,
      { wrapper: Wrapper }
    );
    await waitFor(() => expect(screen.getByLabelText(labels.topUpLink.label).value).toBe('https://example.invalid/current'));
    await act(async () => first.resolve({ data: { success: false, message: 'Synthetic obsolete first mount' } }));
    expect(showError).not.toHaveBeenCalled();
    expect(API.get.mock.calls.filter(([url]) => url === '/api/option/')).toHaveLength(1);
  });
  it('does not mark an omitted unsaved field as saved', async () => {
    const pending = deferred();
    API.get.mockImplementation((url) => (url === '/api/option/' ? pending.promise : Promise.resolve(response([]))));
    API.put.mockResolvedValue(response(null));
    loadStatus.mockResolvedValue(true);
    mount();
    await waitFor(() => expect(API.get).toHaveBeenCalledTimes(2));
    fireEvent.change(screen.getByLabelText(labels.topUpLink.label), { target: { value: 'https://example.invalid/unsaved' } });
    await act(async () => pending.resolve(response([{ key: 'QuotaPerUnit', value: 500000 }])));
    await act(async () => fireEvent.click(screen.getByRole('button', { name: labels.saveButton })));
    expect(API.put).toHaveBeenCalledExactlyOnceWith('/api/option/', { key: 'TopUpLink', value: 'https://example.invalid/unsaved' });
  });
});

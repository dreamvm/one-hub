import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { Wrapper, response, deferred, zh } from './ui-test-utils';
import OperationSetting from '../src/views/Setting/component/OperationSetting';
import StatusProvider, { LoadStatusContext } from '../src/contexts/StatusContext';
import { API } from '../src/utils/api';
import { showSuccess, showError } from '../src/utils/common';
vi.mock('../src/utils/api', () => ({ API: { get: vi.fn(), put: vi.fn() } }));
vi.mock('../src/utils/common', async (original) => ({ ...(await original()), showError: vi.fn(), showSuccess: vi.fn() }));
vi.mock('@iconify/react', () => ({ Icon: () => <span /> }));
vi.mock('../src/views/Setting/component/ChatLinksDataGrid', () => ({ default: () => null }));
const labels = zh.setting_index.operationSettings.generalSettings;
const loadStatus = vi.fn();
let options;
beforeEach(() => {
  vi.clearAllMocks();
  options = {
    TopUpLink: 'https://example.invalid/topup',
    ChatLink: '',
    QuotaPerUnit: 500000,
    RetryTimes: 0,
    RetryCooldownSeconds: 0,
    RetryTimeOut: 0,
    DisplayInCurrencyEnabled: 'false',
    RechargeDiscount: '[]',
    SafeKeyWords: '[]'
  };
  API.get.mockImplementation((url) =>
    Promise.resolve(
      response(
        url === '/api/status'
          ? {
              system_name: 'Synthetic settings QA',
              language: 'zh_CN',
              version: 'v0.0.0',
              quota_per_unit: 500000,
              display_in_currency: false
            }
          : url === '/api/model_ownedby' || url === '/api/option/safe_tools'
            ? []
            : Object.entries(options).map(([key, value]) => ({ key, value }))
      )
    )
  );
  API.put.mockImplementation(async (url, { key, value }) => {
    options[key] = value;
    return response(null);
  });
  loadStatus.mockResolvedValue(true);
});
async function mount(mode = 'light', realProvider = false) {
  render(
    <>
      {realProvider ? (
        <StatusProvider>
          <OperationSetting />
        </StatusProvider>
      ) : (
        <LoadStatusContext.Provider value={loadStatus}>
          <OperationSetting />
        </LoadStatusContext.Provider>
      )}
    </>,
    { wrapper: ({ children }) => <Wrapper mode={mode}>{children}</Wrapper> }
  );
  await waitFor(() => expect(screen.getByLabelText(labels.topUpLink.label).value).toBe(options.TopUpLink));
}
describe('operation settings save outcomes', () => {
  it.each(['light', 'dark'])('saves a normal checkbox and releases loading in %s', async (mode) => {
    await mount(mode);
    fireEvent.click(screen.getByRole('checkbox', { name: labels.displayInCurrency }));
    await waitFor(() => expect(showSuccess).toHaveBeenCalledWith('设置成功！'));
    expect(API.put).toHaveBeenCalledWith('/api/option/', { key: 'DisplayInCurrencyEnabled', value: 'true' });
    expect(screen.getByRole('checkbox', { name: labels.displayInCurrency }).checked).toBe(true);
    expect(screen.getByLabelText(labels.topUpLink.label).disabled).toBe(false);
  });
  it.each(['business', 'rejection', 'undefined'])('does not claim success or remain disabled on checkbox %s', async (mode) => {
    await mount();
    if (mode === 'business') API.put.mockResolvedValue({ data: { success: false, message: 'Synthetic rejected' } });
    if (mode === 'rejection') API.put.mockRejectedValue(new Error('Synthetic unavailable'));
    if (mode === 'undefined') API.put.mockResolvedValue(undefined);
    await act(async () => fireEvent.click(screen.getByRole('checkbox', { name: labels.displayInCurrency })));
    expect(showSuccess).not.toHaveBeenCalled();
    expect(screen.getByRole('checkbox', { name: labels.displayInCurrency }).checked).toBe(false);
    expect(screen.getByLabelText(labels.topUpLink.label).disabled).toBe(false);
  });
  it.each(['business', 'rejection', 'undefined'])('stops a multi-setting save on %s without claiming success', async (mode) => {
    await mount();
    fireEvent.change(screen.getByLabelText(labels.topUpLink.label), { target: { value: 'https://example.invalid/new' } });
    fireEvent.change(screen.getByLabelText(labels.chatLink.label), { target: { value: 'https://example.invalid/chat' } });
    if (mode === 'business') API.put.mockResolvedValue({ data: { success: false, message: 'Synthetic rejected' } });
    if (mode === 'rejection') API.put.mockRejectedValue(new Error('Synthetic unavailable'));
    if (mode === 'undefined') API.put.mockResolvedValue(undefined);
    await act(async () => fireEvent.click(screen.getByRole('button', { name: labels.saveButton })));
    expect(API.put).toHaveBeenCalledTimes(1);
    expect(showSuccess).not.toHaveBeenCalled();
    expect(showError).toHaveBeenCalled();
    expect(screen.getByLabelText(labels.topUpLink.label).disabled).toBe(false);
    expect(screen.getByLabelText(labels.chatLink.label).value).toBe('https://example.invalid/chat');
  });
  it('keeps save controls disabled until all acknowledged writes complete', async () => {
    await mount();
    fireEvent.change(screen.getByLabelText(labels.topUpLink.label), { target: { value: 'https://example.invalid/new' } });
    fireEvent.change(screen.getByLabelText(labels.chatLink.label), { target: { value: 'https://example.invalid/chat' } });
    const first = deferred(),
      second = deferred();
    API.put.mockImplementationOnce(() => first.promise).mockImplementationOnce(() => second.promise);
    const saveButton = screen.getByRole('button', { name: labels.saveButton });
    const currencyCheckbox = screen.getByRole('checkbox', { name: labels.displayInCurrency });
    fireEvent.click(saveButton);
    expect(saveButton.disabled).toBe(true);
    expect(currencyCheckbox.disabled).toBe(true);
    await act(async () => {
      options.TopUpLink = 'https://example.invalid/new';
      first.resolve(response(null));
    });
    expect(API.put).toHaveBeenCalledTimes(2);
    expect(screen.getByLabelText(labels.topUpLink.label).disabled).toBe(true);
    expect(showSuccess).not.toHaveBeenCalled();
    await act(async () => {
      options.ChatLink = 'https://example.invalid/chat';
      second.resolve(response(null));
    });
    expect(showSuccess).toHaveBeenCalledExactlyOnceWith('保存成功！');
    expect(loadStatus).toHaveBeenCalledTimes(1);
    expect(document.body.contains(saveButton)).toBe(true);
    expect(saveButton.disabled).toBe(false);
    // This mounts the full settings form and completes two separate writes on a shared CI runner.
  }, 15000);
  it('retains an acknowledged first write when a later write fails', async () => {
    await mount();
    fireEvent.change(screen.getByLabelText(labels.topUpLink.label), { target: { value: 'https://example.invalid/new' } });
    fireEvent.change(screen.getByLabelText(labels.chatLink.label), { target: { value: 'https://example.invalid/chat' } });
    API.put
      .mockImplementationOnce(async (url, { key, value }) => {
        options[key] = value;
        return response(null);
      })
      .mockResolvedValueOnce({ data: { success: false, message: 'Second rejected' } });
    await act(async () => fireEvent.click(screen.getByRole('button', { name: labels.saveButton })));
    expect(options.TopUpLink).toBe('https://example.invalid/new');
    expect(options.ChatLink).toBe('');
    expect(showSuccess).not.toHaveBeenCalled();
    expect(showError).toHaveBeenCalledWith('部分设置已保存，其余设置未全部保存：Second rejected');
    expect(screen.getByLabelText(labels.chatLink.label).value).toBe('https://example.invalid/chat');
    expect(screen.getByLabelText(labels.topUpLink.label).disabled).toBe(false);
  });
  it('distinguishes a saved value from a failed status refresh', async () => {
    await mount();
    loadStatus.mockRejectedValue(new Error('Synthetic refresh unavailable'));
    await act(async () => fireEvent.click(screen.getByRole('checkbox', { name: labels.displayInCurrency })));
    expect(options.DisplayInCurrencyEnabled).toBe('true');
    expect(screen.getByRole('checkbox', { name: labels.displayInCurrency }).checked).toBe(true);
    expect(showError).toHaveBeenCalledWith('设置已保存，但状态刷新失败：Synthetic refresh unavailable');
    expect(screen.getByLabelText(labels.topUpLink.label).disabled).toBe(false);
  });
  it.each(['payment', 'claude'])('validates the full %s group before any write', async (group) => {
    await mount();
    const payment = zh.setting_index.operationSettings.paymentSettings;
    const claude = zh.setting_index.operationSettings.claudeSettings;
    const scalar = group === 'payment' ? payment.usdRate.label : claude.budgetTokensPercentage.label;
    const json = group === 'payment' ? payment.discount.label : claude.defaultMaxTokens.label;
    fireEvent.change(screen.getByLabelText(scalar), { target: { value: '25' } });
    fireEvent.change(screen.getByLabelText(json), { target: { value: 'invalid-json' } });
    await act(async () => fireEvent.click(screen.getByRole('button', { name: group === 'payment' ? payment.save : claude.save })));
    expect(API.put).not.toHaveBeenCalled();
    expect(showSuccess).not.toHaveBeenCalled();
    expect(showError).toHaveBeenCalled();
    expect(screen.getByLabelText(scalar).disabled).toBe(false);
  });
  it.each(['business', 'rejection', 'undefined'])('reports acknowledged save with failed option readback: %s', async (mode) => {
    await mount();
    if (mode === 'business') API.get.mockResolvedValueOnce({ data: { success: false, message: 'Synthetic readback failure' } });
    if (mode === 'rejection') API.get.mockRejectedValueOnce(new Error('Synthetic readback unavailable'));
    if (mode === 'undefined') API.get.mockResolvedValueOnce(undefined);
    await act(async () => fireEvent.click(screen.getByRole('checkbox', { name: labels.displayInCurrency })));
    expect(options.DisplayInCurrencyEnabled).toBe('true');
    expect(showSuccess).not.toHaveBeenCalled();
    expect(showError).toHaveBeenCalledWith(expect.stringContaining('设置已保存，但状态刷新失败'));
    expect(screen.getByLabelText(labels.topUpLink.label).disabled).toBe(false);
  });
  it.each(['business', 'rejection', 'undefined'])('reports real provider refresh failure after a saved checkbox: %s', async (mode) => {
    await mount('light', true);
    const original = API.get.getMockImplementation();
    API.get.mockImplementation((url) => {
      if (url !== '/api/status') return original(url);
      if (mode === 'rejection') return Promise.reject(new Error('Synthetic status unavailable'));
      return Promise.resolve(mode === 'undefined' ? undefined : { data: { success: false, message: 'Synthetic status failure' } });
    });
    await act(async () => fireEvent.click(screen.getByRole('checkbox', { name: labels.displayInCurrency })));
    expect(options.DisplayInCurrencyEnabled).toBe('true');
    expect(showSuccess).not.toHaveBeenCalled();
    expect(showError).toHaveBeenCalledWith(expect.stringContaining('设置已保存，但状态刷新失败'));
    expect(screen.getByLabelText(labels.topUpLink.label).disabled).toBe(false);
  });
  it('accepts a successful refresh from the real status provider', async () => {
    await mount('light', true);
    await act(async () => fireEvent.click(screen.getByRole('checkbox', { name: labels.displayInCurrency })));
    expect(showSuccess).toHaveBeenCalledWith('设置成功！');
    expect(screen.getByRole('checkbox', { name: labels.displayInCurrency }).checked).toBe(true);
  });
  it.each(['business', 'rejection', 'undefined'])('reports saved group with failed tools refresh: %s', async (mode) => {
    await mount();
    const original = API.get.getMockImplementation();
    API.get.mockImplementation((url) => {
      if (url !== '/api/option/safe_tools') return original(url);
      if (mode === 'rejection') return Promise.reject(new Error('Synthetic tools unavailable'));
      return Promise.resolve(mode === 'undefined' ? undefined : { data: { success: false, message: 'Synthetic tools failure' } });
    });
    const errorLog = vi.spyOn(console, 'error').mockImplementation(() => {});
    try {
      fireEvent.change(screen.getByLabelText(labels.topUpLink.label), { target: { value: 'https://example.invalid/changed' } });
      await act(async () => fireEvent.click(screen.getByRole('button', { name: labels.saveButton })));
      expect(options.TopUpLink).toBe('https://example.invalid/changed');
      expect(showSuccess).not.toHaveBeenCalled();
      expect(showError).toHaveBeenCalledWith(expect.stringContaining('设置已保存，但状态刷新失败'));
      expect(screen.getByLabelText(labels.topUpLink.label).disabled).toBe(false);
    } finally {
      errorLog.mockRestore();
    }
  });
});

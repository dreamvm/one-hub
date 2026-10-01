import React from 'react';
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { act, render, screen, cleanup, fireEvent } from '@testing-library/react';
import { Wrapper, response, deferred } from './ui-test-utils';
import PayDialog from '../src/views/Topup/component/PayDialog';
import { showError } from '../src/utils/common';
import { API } from '../src/utils/api';

vi.mock('../src/utils/api', () => ({ API: { get: vi.fn(), post: vi.fn() } }));
vi.mock('../src/utils/common', () => ({ showError: vi.fn() }));
vi.mock('react-qrcode-logo', () => ({ QRCode: () => <span>synthetic QR</span> }));
const order = (trade = 'synthetic-order') => response({ type: 2, trade_no: trade, data: { url: 'https://example.invalid/synthetic' } });
const pendingStatus = { data: { success: false } };
const tick = (ms = 3000) =>
  act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
function mount(props = {}) {
  return render(<PayDialog open amount={10} uuid="synthetic-gateway" onClose={() => {}} {...props} />, { wrapper: Wrapper });
}
beforeEach(() => {
  vi.useFakeTimers();
  vi.clearAllMocks();
  API.post.mockResolvedValue(order());
  API.get.mockResolvedValue(pendingStatus);
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.clearAllTimers();
  vi.useRealTimers();
});

describe('payment dialog lifecycle', () => {
  it('creates one order and polls until confirmed success', async () => {
    API.get.mockResolvedValueOnce(pendingStatus).mockResolvedValueOnce({ data: { success: true } });
    mount();
    await act(async () => {});
    expect(API.post).toHaveBeenCalledExactlyOnceWith('/api/user/order', { uuid: 'synthetic-gateway', amount: 10 });
    expect(screen.getByText('请扫码支付')).toBeTruthy();
    await tick();
    await tick();
    expect(screen.getByText('支付成功')).toBeTruthy();
    await tick(6000);
    expect(API.get).toHaveBeenCalledTimes(2);
  });
  it('does not create another order when the parent callback identity changes', async () => {
    const view = mount();
    await act(async () => {});
    view.rerender(<PayDialog open amount={10} uuid="synthetic-gateway" onClose={() => {}} />);
    await act(async () => {});
    expect(API.post).toHaveBeenCalledTimes(1);
  });
  it('does not start polling for an order returned after the dialog closes', async () => {
    const pending = deferred();
    API.post.mockReturnValue(pending.promise);
    const close = vi.fn();
    const view = mount({ onClose: close });
    view.rerender(<PayDialog open={false} amount={10} uuid="synthetic-gateway" onClose={close} />);
    await act(async () => pending.resolve(order()));
    await tick(6000);
    expect(API.get).not.toHaveBeenCalled();
  });
  it('stops an active poll after unmount', async () => {
    const view = mount();
    await act(async () => {});
    await tick();
    expect(API.get).toHaveBeenCalledTimes(1);
    view.unmount();
    await tick(6000);
    expect(API.get).toHaveBeenCalledTimes(1);
  });
  it('ignores an old status response after reopening for a new order', async () => {
    const firstStatus = deferred();
    API.get.mockReturnValueOnce(firstStatus.promise);
    const close = vi.fn();
    const view = mount({ onClose: close });
    await act(async () => {});
    await tick();
    view.rerender(<PayDialog open={false} amount={10} uuid="synthetic-gateway" onClose={close} />);
    API.post.mockResolvedValue(order('new-synthetic-order'));
    view.rerender(<PayDialog open amount={10} uuid="synthetic-gateway" onClose={close} />);
    await act(async () => {});
    await act(async () => firstStatus.resolve({ data: { success: true } }));
    expect(screen.queryByText('支付成功')).toBeNull();
    expect(screen.getByText('请扫码支付')).toBeTruthy();
  });
  it('does not overlap slow status requests', async () => {
    const status = deferred();
    API.get.mockReturnValueOnce(status.promise);
    mount();
    await act(async () => {});
    await tick(9000);
    expect(API.get).toHaveBeenCalledTimes(1);
    await act(async () => status.resolve(pendingStatus));
    await tick();
    expect(API.get).toHaveBeenCalledTimes(2);
  });
  it('retries status failure without creating another order or claiming payment', async () => {
    API.get.mockRejectedValueOnce(new Error('synthetic unavailable')).mockResolvedValueOnce({ data: { success: true } });
    mount();
    await act(async () => {});
    await tick();
    expect(screen.getByRole('status').textContent).toContain('暂时无法查询');
    expect(screen.queryByText('支付成功')).toBeNull();
    await tick();
    expect(screen.getByText('支付成功')).toBeTruthy();
    expect(screen.queryByRole('status')).toBeNull();
    expect(API.post).toHaveBeenCalledTimes(1);
  });
  it('shows uncertain order creation without automatically retrying', async () => {
    API.post.mockRejectedValueOnce(new Error('synthetic lost response'));
    mount();
    await act(async () => {});
    expect(screen.getByText('无法确认订单创建结果')).toBeTruthy();
    expect(screen.queryByAltText('loading')).toBeNull();
    await tick(9000);
    expect(API.post).toHaveBeenCalledTimes(1);
    expect(API.get).not.toHaveBeenCalled();
  });
  it('closes a rejected order using the current parent callback', async () => {
    const pending = deferred();
    API.post.mockReturnValueOnce(pending.promise);
    const oldClose = vi.fn(),
      newClose = vi.fn();
    const view = mount({ onClose: oldClose });
    view.rerender(<PayDialog open amount={10} uuid="synthetic-gateway" onClose={newClose} />);
    await act(async () => pending.resolve({ data: { success: false, message: 'synthetic rejection' } }));
    expect(showError).toHaveBeenCalledWith('synthetic rejection');
    expect(newClose).toHaveBeenCalledTimes(1);
    expect(oldClose).not.toHaveBeenCalled();
    await tick();
    expect(API.get).not.toHaveBeenCalled();
    expect(API.post).toHaveBeenCalledTimes(1);
  });
  it('cancels immediately on the close button even before the parent closes', async () => {
    const pending = deferred();
    API.post.mockReturnValueOnce(pending.promise);
    const close = vi.fn();
    mount({ onClose: close });
    fireEvent.click(screen.getByRole('button', { name: 'close' }));
    await act(async () => pending.resolve(order()));
    await tick(6000);
    expect(close).toHaveBeenCalledTimes(1);
    expect(API.get).not.toHaveBeenCalled();
    expect(screen.queryByText('请扫码支付')).toBeNull();
  });
  it('ignores late creation when the amount changes', async () => {
    const pending = deferred();
    API.post.mockReturnValueOnce(pending.promise);
    const close = vi.fn();
    const view = mount({ onClose: close });
    view.rerender(<PayDialog open amount={20} uuid="synthetic-gateway" onClose={close} />);
    await act(async () => {});
    await act(async () => pending.resolve(order('old-synthetic-order')));
    await tick();
    expect(API.post).toHaveBeenCalledTimes(2);
    expect(API.get).toHaveBeenCalledExactlyOnceWith('/api/user/order/status?trade_no=synthetic-order');
  });
  it.each(['POST', 'GET'])('preserves the %s redirect and manual link while polling', async (method) => {
    const submit = vi.spyOn(HTMLFormElement.prototype, 'submit').mockImplementation(() => {});
    API.post.mockResolvedValueOnce(
      response({
        type: 1,
        trade_no: 'redirect-order',
        data: {
          method,
          url: 'https://example.invalid/synthetic',
          params: { synthetic: 'value' }
        }
      })
    );
    mount();
    await act(async () => {});
    expect(submit).toHaveBeenCalledTimes(1);
    await tick();
    fireEvent.click(screen.getByText('这里跳转'));
    expect(submit).toHaveBeenCalledTimes(2);
  });
});

import React from 'react';
import PropTypes from 'prop-types';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { Provider } from 'react-redux';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { ThemeProvider } from '@mui/material/styles';
import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import zh from '../src/i18n/locales/zh_CN.json';
import { store } from '../src/store';
import theme from '../src/themes';
import { API } from '../src/utils/api';
import EditModal from '../src/views/Channel/component/EditModal';
import NavCollapse from '../src/layout/MainLayout/Sidebar/MenuList/NavCollapse';

vi.mock('../src/utils/api', () => ({ API: { get: vi.fn(), put: vi.fn(), post: vi.fn() } }));
// Keep real MUI, Formik, routing and i18n; isolate external editors/icons/network.
vi.mock('@monaco-editor/react', () => ({
  default: ({ value, onChange }) => <textarea aria-label="JSON editor" value={value || ''} onChange={(e) => onChange(e.target.value)} />
}));
vi.mock('@iconify/react', async () => {
  const { forwardRef } = await import('react');
  return {
    Icon: forwardRef(function TestIcon(props, ref) {
      return <span ref={ref} />;
    })
  };
});
vi.mock('../src/utils/common', async (original) => ({ ...(await original()), showError: vi.fn(), showSuccess: vi.fn() }));

i18n.use(initReactI18next).init({ lng: 'zh_CN', resources: { zh_CN: { translation: zh } }, interpolation: { escapeValue: false } });
const currentTheme = theme(store.getState().customization);
function Wrapper({ children }) {
  return (
    <Provider store={store}>
      <MemoryRouter>
        <ThemeProvider theme={currentTheme}>{children}</ThemeProvider>
      </MemoryRouter>
    </Provider>
  );
}
Wrapper.propTypes = { children: PropTypes.node };
const channel = (overrides = {}) => ({
  id: 49,
  name: 'Gemini',
  type: 25,
  tag: 'Gemini',
  key: '',
  group: 'default',
  models: 'gemini-test',
  base_url: 'https://example.invalid',
  other: 'v1beta',
  model_mapping: '',
  model_headers: '',
  custom_parameter: '',
  disabled_stream: [],
  pre_cost: 1,
  plugin: { code_execution: { enable: true }, use_openai_api: { enable: false } },
  ...overrides
});
const response = (data = channel()) => ({ data: { success: true, data } });
const props = () => ({
  open: true,
  channelId: 'Gemini',
  isTag: true,
  onCancel: vi.fn(),
  onOk: vi.fn(),
  groupOptions: ['default'],
  modelOptions: [{ id: 'gemini-test', group: 'Gemini' }],
  prices: []
});
const deferred = () => {
  let resolve;
  const promise = new Promise((r) => {
    resolve = r;
  });
  return { promise, resolve };
};
const nameField = () => screen.queryByRole('textbox', { name: /名称/ }) || screen.getByRole('textbox', { name: '标签' });

beforeEach(() => {
  vi.clearAllMocks();
  API.put.mockResolvedValue({ data: { success: true } });
});

describe('channel editing safeguards', () => {
  it('blocks submit while loading and applies a bounded read timeout', async () => {
    const pending = deferred();
    API.get.mockReturnValue(pending.promise);
    render(<EditModal {...props()} />, { wrapper: Wrapper });
    expect(screen.getByRole('button', { name: '提交' }).disabled).toBe(true);
    expect(screen.queryByRole('textbox', { name: /名称/ })).toBeNull();
    expect(API.get).toHaveBeenCalledWith('/api/channel_tag/Gemini', { timeout: 30000 });
    await act(async () => pending.resolve(response()));
    expect(nameField().value).toBe('Gemini');
  });

  it.each(['logical', 'network', 'malformed'])('fails closed on %s failure and supports retry', async (failure) => {
    if (failure === 'network') API.get.mockRejectedValueOnce(new Error('network unavailable'));
    else
      API.get.mockResolvedValueOnce(failure === 'logical' ? { data: { success: false } } : response(channel({ model_mapping: '{broken' })));
    API.get.mockResolvedValueOnce(response());
    render(<EditModal {...props()} />, { wrapper: Wrapper });
    await screen.findByRole('alert');
    expect(screen.getByRole('button', { name: '提交' }).disabled).toBe(true);
    expect(screen.queryByRole('textbox', { name: /名称/ })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: zh.channel_edit.retryLoad }));
    await waitFor(() => expect(nameField().value).toBe('Gemini'));
  });

  it('ignores stale A response and preserves dirty B edits on option refresh', async () => {
    const pending = deferred();
    API.get.mockReturnValueOnce(pending.promise).mockResolvedValueOnce(response(channel({ id: 50, name: 'B', tag: '' })));
    const initial = props();
    const view = render(<EditModal {...initial} />, { wrapper: Wrapper });
    const next = { ...initial, channelId: 50, isTag: false };
    view.rerender(<EditModal {...next} />);
    await waitFor(() => expect(nameField().value).toBe('B'));
    fireEvent.change(nameField(), { target: { value: 'B edited' } });
    await act(async () => pending.resolve(response()));
    view.rerender(<EditModal {...next} modelOptions={[...next.modelOptions, { id: 'new', group: 'Gemini' }]} />);
    expect(nameField().value).toBe('B edited');
    expect(API.get).toHaveBeenCalledTimes(2);
  });

  it('reloads a reopened session and resets grouped-channel restrictions', async () => {
    API.get.mockResolvedValueOnce(response()).mockResolvedValueOnce(response(channel({ name: 'Independent', tag: '' })));
    const initial = { ...props(), channelId: 49, isTag: false };
    const view = render(<EditModal {...initial} />, { wrapper: Wrapper });
    await waitFor(() => expect(nameField().value).toBe('Gemini'));
    const version = () => document.getElementById('channel-other-label');
    expect(version().disabled).toBe(true);
    view.rerender(<EditModal {...initial} open={false} />);
    view.rerender(<EditModal {...initial} />);
    await waitFor(() => expect(nameField().value).toBe('Independent'));
    expect(version().disabled).toBe(false);
  });

  it('opens a blank new channel without making a read request', async () => {
    render(<EditModal {...props()} channelId={0} isTag={false} />, { wrapper: Wrapper });
    await waitFor(() => expect(nameField().value).toBe(''));
    expect(API.get).not.toHaveBeenCalled();
  });

  it('expands plugins independently and preserves values after collapse and save', async () => {
    API.get.mockResolvedValue(response());
    const initial = props();
    render(<EditModal {...initial} />, { wrapper: Wrapper });
    await waitFor(() => expect(nameField().value).toBe('Gemini'));
    const code = screen.getByRole('button', { name: /代码执行/ });
    const compatibility = screen.getByRole('button', { name: /使用OpenAI API/ });
    fireEvent.click(code);
    expect(code.getAttribute('aria-expanded')).toBe('true');
    expect(compatibility.getAttribute('aria-expanded')).toBe('false');
    fireEvent.click(compatibility);
    expect(screen.getByRole('checkbox', { name: /代码执行/ }).checked).toBe(true);
    fireEvent.click(screen.getByRole('checkbox', { name: /使用OpenAI API/ }));
    fireEvent.click(compatibility);
    fireEvent.click(screen.getByRole('button', { name: '提交' }));
    await waitFor(() => expect(API.put).toHaveBeenCalledTimes(1));
    expect(API.put.mock.calls[0][1].plugin).toEqual({ code_execution: { enable: true }, use_openai_api: { enable: true } });
    expect(initial.onOk).toHaveBeenCalledWith(true);
  });
});

const menu = {
  id: 'operation',
  title: '运营',
  type: 'collapse',
  children: [{ id: 'pricing', title: '模型价格', type: 'item', url: '/pricing' }]
};
function Location() {
  return <output data-testid="route">{useLocation().pathname}</output>;
}
describe('mini sidebar navigation', () => {
  it('opens child links, closes on Escape and navigates without a stuck popup', async () => {
    // jsdom has no layout; give MUI a valid anchor rectangle.
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      x: 0,
      y: 0,
      top: 0,
      left: 0,
      bottom: 56,
      right: 80,
      width: 80,
      height: 56,
      toJSON() {}
    });
    render(
      <>
        <NavCollapse menu={menu} level={1} isMini />
        <Location />
      </>,
      { wrapper: Wrapper }
    );
    const trigger = screen.getByRole('button', { name: '运营' });
    fireEvent.click(trigger);
    expect(screen.getByRole('button', { name: '模型价格' }).getAttribute('href')).toBe('/pricing');
    fireEvent.keyDown(screen.getByRole('button', { name: '模型价格' }), { key: 'Escape' });
    await waitFor(() => expect(screen.queryByRole('button', { name: '模型价格' })).toBeNull());
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole('button', { name: '模型价格' }));
    await waitFor(() => expect(screen.queryByRole('button', { name: '模型价格' })).toBeNull());
    expect(screen.getByTestId('route').textContent).toBe('/pricing');
  });
});

import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import SupportModels from '../src/views/Dashboard/component/SupportModels';
import { API } from '../src/utils/api';
import { copy, showError } from '../src/utils/common';
import { store } from '../src/store';
import { SET_MODEL_OWNEDBY } from '../src/store/actions';
import { Wrapper, deferred } from './ui-test-utils';

vi.mock('../src/utils/api', () => ({ API: { get: vi.fn() } }));
vi.mock('../src/utils/common', async (original) => ({ ...(await original()), showError: vi.fn(), copy: vi.fn() }));
const models = { 'b-z': { owned_by: 'Beta' }, 'a-one': { owned_by: 'Alpha' }, 'b-a': { owned_by: 'Beta' } };
const owners = [
  { name: 'Alpha', id: 1 },
  { name: 'Beta', id: 2 }
];
beforeEach(() => {
  API.get.mockReset().mockResolvedValue({ data: { success: true, data: models } });
  copy.mockReset();
  showError.mockReset();
  store.dispatch({ type: SET_MODEL_OWNEDBY, payload: [] });
});
describe('supported models and asynchronous owner metadata', () => {
  it('keeps numeric provider names in their metadata order', async () => {
    API.get.mockResolvedValue({
      data: { success: true, data: { '360-model': { owned_by: '360' }, 'openai-model': { owned_by: 'OpenAI' } } }
    });
    store.dispatch({
      type: SET_MODEL_OWNEDBY,
      payload: [
        { name: 'OpenAI', id: 1 },
        { name: '360', id: 19 }
      ]
    });
    render(
      <Wrapper>
        <SupportModels />
      </Wrapper>
    );
    expect(await screen.findByText('OpenAI:')).toBeTruthy();
    fireEvent.click(screen.getByRole('button'));
    expect(screen.getByText('OpenAI').compareDocumentPosition(screen.getByText('360')) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
  it.each(['light', 'dark'])('keeps known owner order, expansion and copying in %s mode', async (mode) => {
    store.dispatch({ type: SET_MODEL_OWNEDBY, payload: owners });
    render(
      <Wrapper mode={mode}>
        <SupportModels />
      </Wrapper>
    );
    expect(await screen.findByText('Alpha:')).toBeTruthy();
    expect(screen.queryByText('b-a')).toBeNull();
    fireEvent.click(screen.getByRole('button'));
    expect(screen.getByText('b-a')).toBeTruthy();
    expect(screen.getByText('b-a').compareDocumentPosition(screen.getByText('b-z')) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    fireEvent.click(screen.getByText('b-a'));
    expect(copy).toHaveBeenCalledWith('b-a', expect.any(String));
    expect(API.get).toHaveBeenCalledTimes(1);
  });
  it('reorders an already loaded model list when owner metadata arrives', async () => {
    render(
      <Wrapper>
        <SupportModels />
      </Wrapper>
    );
    expect(await screen.findByText('Beta:')).toBeTruthy();
    act(() => store.dispatch({ type: SET_MODEL_OWNEDBY, payload: owners }));
    await waitFor(() => expect(screen.getByText('Alpha:')).toBeTruthy());
    expect(API.get).toHaveBeenCalledTimes(1);
  });
  it('uses owner metadata that arrived while the model request was pending', async () => {
    const pending = deferred();
    API.get.mockReturnValue(pending.promise);
    render(
      <Wrapper>
        <SupportModels />
      </Wrapper>
    );
    act(() => store.dispatch({ type: SET_MODEL_OWNEDBY, payload: owners }));
    await act(async () => pending.resolve({ data: { success: true, data: models } }));
    expect(await screen.findByText('Alpha:')).toBeTruthy();
    expect(API.get).toHaveBeenCalledTimes(1);
  });
  it('keeps the expanded list and uses updated owner order without another request', async () => {
    store.dispatch({ type: SET_MODEL_OWNEDBY, payload: owners });
    render(
      <Wrapper>
        <SupportModels />
      </Wrapper>
    );
    await screen.findByText('Alpha:');
    fireEvent.click(screen.getByRole('button'));
    act(() =>
      store.dispatch({
        type: SET_MODEL_OWNEDBY,
        payload: [
          { name: 'Beta', id: 1 },
          { name: 'Alpha', id: 2 }
        ]
      })
    );
    expect(screen.getByText('Beta').compareDocumentPosition(screen.getByText('Alpha')) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(screen.getByText('a-one')).toBeTruthy();
    expect(API.get).toHaveBeenCalledTimes(1);
  });
  it('ignores an obsolete request result from a cleaned-up effect', async () => {
    const old = deferred();
    API.get.mockReturnValueOnce(old.promise).mockResolvedValueOnce({ data: { success: true, data: models } });
    store.dispatch({ type: SET_MODEL_OWNEDBY, payload: owners });
    render(
      <React.StrictMode>
        <Wrapper>
          <SupportModels />
        </Wrapper>
      </React.StrictMode>
    );
    await screen.findByText('Alpha:');
    await act(async () => old.resolve({ data: { success: true, data: { obsolete: { owned_by: 'Old' } } } }));
    expect(screen.getByText('Alpha:')).toBeTruthy();
    expect(screen.queryByText('obsolete')).toBeNull();
    expect(API.get).toHaveBeenCalledTimes(2);
  });
  it('retains existing failure notification without rendering model data', async () => {
    API.get.mockRejectedValue(new Error('Synthetic model service unavailable'));
    render(
      <Wrapper>
        <SupportModels />
      </Wrapper>
    );
    await waitFor(() => expect(showError).toHaveBeenCalledWith('Synthetic model service unavailable'));
    expect(screen.queryByText('a-one')).toBeNull();
  });
});

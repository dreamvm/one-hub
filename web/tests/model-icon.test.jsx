import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Formik } from 'formik';
import { Wrapper } from './ui-test-utils';
import ModelLimitSelector from '../src/views/Token/component/ModelLimitSelector';
import unknownModelIcon from '../src/assets/images/icons/unknown_type.svg';

describe('bundled model icon fallback', () => {
  it.each(['light', 'dark'])('falls back once and still allows model selection in %s mode', async (mode) => {
    const setter = vi.spyOn(HTMLImageElement.prototype, 'src', 'set');
    render(
      <Wrapper mode={mode}>
        <Formik initialValues={{ setting: { limits: { limit_model_setting: { models: [] } } } }} onSubmit={() => {}}>
          {({ values }) => (
            <>
              <ModelLimitSelector
                modelOptions={[{ id: 'gemini-test', name: 'gemini-test', owned_by: 'Gemini', groups: ['default'] }]}
                getModelIcon={() => '/invalid-provider-logo.svg'}
              />
              <output data-testid="selected-models">{JSON.stringify(values.setting.limits.limit_model_setting.models)}</output>
            </>
          )}
        </Formik>
      </Wrapper>
    );
    fireEvent.mouseDown(screen.getByRole('combobox'));
    const logo = screen.getByRole('img', { name: 'Gemini' });
    setter.mockClear();
    fireEvent.error(logo);
    expect(logo.getAttribute('src')).toBe(unknownModelIcon);
    expect(setter).toHaveBeenCalledTimes(1);
    fireEvent.error(logo);
    expect(setter).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole('option'));
    await waitFor(() => expect(screen.getByTestId('selected-models').textContent).toBe('["gemini-test"]'));
  });
});

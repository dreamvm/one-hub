import React from 'react';
import PropTypes from 'prop-types';
import { Provider } from 'react-redux';
import { MemoryRouter } from 'react-router-dom';
import { ThemeProvider } from '@mui/material/styles';
import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import zh from '../src/i18n/locales/zh_CN.json';
import { store } from '../src/store';
import theme from '../src/themes';

i18n.use(initReactI18next).init({ lng: 'zh_CN', resources: { zh_CN: { translation: zh } }, interpolation: { escapeValue: false } });
export function Wrapper({ children, mode = 'light' }) {
  return (
    <Provider store={store}>
      <MemoryRouter>
        <ThemeProvider theme={theme({ ...store.getState().customization, theme: mode })}>{children}</ThemeProvider>
      </MemoryRouter>
    </Provider>
  );
}
Wrapper.propTypes = { children: PropTypes.node, mode: PropTypes.string };
export const response = (data) => ({ data: { success: true, data } });
export const deferred = () => {
  let resolve, reject;
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
};
export { zh };

import React from 'react';
import { afterEach, describe, expect, it } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import NotFoundView from '../src/views/Error';
import { zh } from './ui-test-utils';

afterEach(() => window.history.replaceState({ idx: 0 }, '', '/'));

function renderPage(mode) {
  return render(
    <ThemeProvider theme={createTheme({ palette: { mode } })}>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<h1>Synthetic home</h1>} />
          <Route path="/synthetic-previous" element={<h1>Synthetic previous page</h1>} />
          <Route path="*" element={<NotFoundView />} />
        </Routes>
      </BrowserRouter>
    </ThemeProvider>
  );
}

describe.each(['light', 'dark'])('not-found return in %s mode', (mode) => {
  it('returns to the previous application page', async () => {
    window.history.replaceState({ idx: 0 }, '', '/synthetic-previous');
    window.history.pushState({ idx: 1 }, '', '/synthetic-missing');
    renderPage(mode);
    fireEvent.click(screen.getByRole('button', { name: zh.common.back }));
    expect(await screen.findByRole('heading', { name: 'Synthetic previous page' })).toBeTruthy();
    expect(window.location.pathname).toBe('/synthetic-previous');
  });

  it('replaces a directly opened missing route with the home page', async () => {
    window.history.replaceState({ idx: 0 }, '', '/synthetic-missing');
    renderPage(mode);
    fireEvent.click(screen.getByRole('button', { name: zh.common.back }));
    await waitFor(() => expect(window.location.pathname).toBe('/'));
    expect(screen.getByRole('heading', { name: 'Synthetic home' })).toBeTruthy();
    expect(window.history.state.idx).toBe(0);
  });
});

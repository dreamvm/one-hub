import { defineConfig, mergeConfig } from 'vitest/config';
import viteConfig from './vite.config.mjs';

export default mergeConfig(
  viteConfig,
  defineConfig({
    server: { open: false },
    test: {
      environment: 'jsdom',
      css: { include: /\.scss$/ },
      setupFiles: ['./tests/setup.js'],
      include: ['tests/**/*.test.jsx'],
      restoreMocks: true
    }
  })
);

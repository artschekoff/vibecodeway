/// <reference types="vitest/config" />
import { defineConfig } from 'vitest/config';
import path from 'path';

export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: [path.resolve(__dirname, 'src/test-setup.ts')],
    css: true,
  },
});

/// <reference types="vitest/config" />
import { defineConfig } from 'vitest/config';
import path from 'path';

export default defineConfig({
  resolve: {
    alias: [
      { find: /.*\/wailsjs\/go\/main\/App/, replacement: path.resolve(__dirname, 'src/__mocks__/wailsjs-app.ts') },
      { find: '@', replacement: path.resolve(__dirname, './src') },
      { find: 'react', replacement: path.resolve(__dirname, '../node_modules/react') },
      { find: 'react-dom', replacement: path.resolve(__dirname, '../node_modules/react-dom') },
    ],
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: [path.resolve(__dirname, 'src/test-setup.ts')],
    css: true,
  },
});

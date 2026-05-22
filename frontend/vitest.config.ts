/// <reference types="vitest/config" />
import { defineConfig } from 'vitest/config';
import path from 'path';

const mainFrontend = path.resolve(__dirname, 'node_modules');

export default defineConfig({
  resolve: {
    alias: [
      { find: /.*\/wailsjs\/go\/main\/App/, replacement: path.resolve(__dirname, 'src/__mocks__/wailsjs-app.ts') },
      { find: '@', replacement: path.resolve(__dirname, './src') },
      // Pin react, react-dom, and lucide-react to the main project's copies to avoid dual-React issues
      { find: /^react\/(.+)$/, replacement: path.resolve(mainFrontend, 'react/$1') },
      { find: /^react$/, replacement: path.resolve(mainFrontend, 'react') },
      { find: /^react-dom\/(.+)$/, replacement: path.resolve(mainFrontend, 'react-dom/$1') },
      { find: /^react-dom$/, replacement: path.resolve(mainFrontend, 'react-dom') },
      { find: /^lucide-react$/, replacement: path.resolve(mainFrontend, 'lucide-react') },
    ],
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: [path.resolve(__dirname, 'src/test-setup.ts')],
    css: true,
  },
});

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
      // monaco-editor's `?worker` imports rely on Vite's client-only worker
      // plugin, which vitest's SSR module transform does not run. Redirect
      // them to a stub so modules using these imports can load under test.
      { find: /^monaco-editor\/esm\/vs\/.*\.worker(\.js)?\?worker$/, replacement: path.resolve(__dirname, 'src/__mocks__/monaco-worker-stub.ts') },
    ],
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: [path.resolve(__dirname, 'src/test-setup.ts')],
    css: true,
    // The userEvent-driven interaction tests are slow by nature: the heaviest
    // (ContextForm — Autodetect PR) measures ~1.1s idle, and vitest runs 118
    // files across workers that already saturate the CPU. Anything else running
    // on the machine — a dev server, a browser, a second suite — starves them
    // past the 5s default, and they fail with "Test timed out in 5000ms"
    // rather than an assertion. Reproduced deterministically by running two
    // suites at once: 8-11 timeouts spread across 5 files, zero assertion
    // failures. 15s keeps ~14x headroom over the slowest test while still
    // failing a genuinely hung test inside a normal run.
    testTimeout: 15000,
    hookTimeout: 15000,
  },
});

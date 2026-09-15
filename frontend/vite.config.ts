import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'
import path from 'node:path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: [
      // monaco-vim's exports map lists "browser" first, which points at its UMD
      // bundle — vite picks it for browser builds, and the UMD's exports end up
      // wrapped under `default` with a broken monaco reference in production.
      // Force the clean ESM build (not exposed via the exports map, hence the
      // direct file path).
      {
        find: /^monaco-vim$/,
        replacement: path.resolve(__dirname, "node_modules/monaco-vim/dist/index.mjs"),
      },
      // monaco-vim's ESM build imports monaco internals under monaco-editor/esm/…,
      // which monaco-editor@0.56's exports map doesn't expose — resolve the whole
      // prefix straight to the files on disk (vite appends extensions itself).
      {
        find: /^monaco-editor\/esm\//,
        replacement: path.resolve(__dirname, "node_modules/monaco-editor/esm") + "/",
      },
    ],
  },
})

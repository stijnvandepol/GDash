/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { legacyCss } from './build/legacyCss.ts';

export default defineConfig({
  plugins: [svelte(), legacyCss()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    // Cast receivers run an older Chromium than desktop browsers. ES2020 is
    // Chrome 80, the oldest Cast runtime still in use; the CSS minifier
    // takes the same browsers (it writes inset as top, right, bottom and
    // left). What a target cannot lower, container queries above all, the
    // legacy stylesheets of build/legacyCss.ts cover.
    target: 'es2020',
    rollupOptions: {
      input: {
        admin: 'admin/index.html',
        display: 'display/index.html',
        preview: 'preview/index.html',
        receiver: 'receiver/index.html',
      },
    },
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8420',
    },
  },
  test: {
    include: ['src/**/*.test.ts', 'build/**/*.test.ts'],
    // Fixed, so date tests cross a real daylight saving change.
    env: { TZ: 'Europe/Amsterdam' },
  },
});

/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    // Cast receivers run an older Chromium than desktop browsers. ES2020 is
    // a conservative target until real devices have been measured.
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
    include: ['src/**/*.test.ts'],
    // Fixed, so date tests cross a real daylight saving change.
    env: { TZ: 'Europe/Amsterdam' },
  },
});

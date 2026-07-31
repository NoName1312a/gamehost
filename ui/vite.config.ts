/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: { fs: { allow: ['..'] } },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    // src/lib/supabase.ts builds its client at module load, so any suite that
    // transitively imports it dies with "supabaseUrl is required" when these
    // are unset. They come from .env, which is gitignored — so the suite passed
    // only on machines that happened to have one, and four suites silently
    // failed on a clean clone and in CI. Tests mock every Supabase call, so
    // these need to be well-formed, never real. Suites asserting on the values
    // override them with vi.stubEnv.
    env: {
      VITE_SUPABASE_URL: 'http://localhost:54321',
      VITE_SUPABASE_ANON_KEY: 'test-anon-key',
    },
  },
})

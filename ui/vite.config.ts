import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  // Allow importing the repo-root CHANGELOG.md (one level above ui/) via ?raw in
  // the dev server; the production build can read it regardless.
  server: { fs: { allow: ['..'] } },
})

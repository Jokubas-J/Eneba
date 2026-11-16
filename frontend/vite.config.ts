import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  // The third argument '' loads every variable from .env files, not only those
  // prefixed with VITE_. API_PROXY_TARGET deliberately has no VITE_ prefix: it is
  // needed only by this dev server and must not be compiled into the browser bundle.
  const env = loadEnv(mode, process.cwd(), '')

  return {
    plugins: [react(), tailwindcss()],
    server: {
      // In production Vercel forwards /api/* to the Go server (see vercel.json).
      // The dev server does the same, so the browser always calls its own origin:
      // no CORS, first-party cookies, and identical URLs in both environments.
      proxy: {
        '/api': env.API_PROXY_TARGET || 'http://localhost:8080',
      },
    },
  }
})

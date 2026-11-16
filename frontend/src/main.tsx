import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router'
// The three weights Eneba's pages load. The font files are bundled with the app
// (self-hosted), so no request goes to a font service.
import '@fontsource/metropolis/400.css'
import '@fontsource/metropolis/600.css'
import '@fontsource/metropolis/800.css'
import './index.css'
import App from './App.tsx'

// One QueryClient for the whole app: it owns the cache of server responses, so two
// components asking for the same data share one request.
const queryClient = new QueryClient()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      {/* BrowserRouter keeps the UI in sync with the address bar using normal URLs
          (/store, not /#/store); vercel.json sends every such URL to index.html. */}
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
)

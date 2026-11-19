import { QueryClientProvider } from '@tanstack/react-query'
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
import { CartProvider } from './cart/CartContext.tsx'
import { ErrorBoundary } from './components/ErrorBoundary.tsx'
import { WakeUpNotice } from './components/WakeUpNotice.tsx'
import { queryClient } from './queryClient.ts'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    {/* The last line of defence: a crash anywhere shows a message, not a blank page. */}
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        {/* BrowserRouter keeps the UI in sync with the address bar using normal URLs
            (/store, not /#/store); vercel.json sends every such URL to index.html. */}
        <BrowserRouter>
          {/* Inside the router, because the "Added to cart" dialog navigates. */}
          <CartProvider>
            <App />
          </CartProvider>
        </BrowserRouter>
        {/* Says "waking up the server" when data takes unusually long (cold start). */}
        <WakeUpNotice />
      </QueryClientProvider>
    </ErrorBoundary>
  </StrictMode>,
)

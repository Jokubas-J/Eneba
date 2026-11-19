import { type ReactNode, Suspense } from 'react'
import { Spinner } from './Spinner.tsx'

// Pages other than the homepage are loaded on demand (React.lazy in App.tsx): the
// browser downloads a page's code the first time it is visited. While that small
// file arrives, the layout (header, footer) stays and this shows in the page's place.
export function PageSuspense({ children }: { children: ReactNode }) {
  return (
    <Suspense
      fallback={
        <div className="flex justify-center py-16" role="status">
          <Spinner className="size-8 text-text-muted" />
          <span className="sr-only">Loading the page…</span>
        </div>
      }
    >
      {children}
    </Suspense>
  )
}

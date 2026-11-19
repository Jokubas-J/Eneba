import { useIsFetching } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { Spinner } from './Spinner.tsx'

// After this long with data still loading, the notice appears.
const showAfterMs = 4000

// "Waking up the server…" (PLAN.md §7, risk "Render cold start"). The demo's API runs
// on Render's free tier, which sleeps after 15 idle minutes and needs up to a minute
// to start again. Pages show their skeletons meanwhile; this notice says why it takes
// so long, so a first-time visitor does not think the site is broken.
//
// useIsFetching counts the server requests TanStack Query has in flight, from every
// component. A normal page load finishes long before 4 seconds and never shows it.
export function WakeUpNotice() {
  const busy = useIsFetching() > 0
  const [slow, setSlow] = useState(false)

  useEffect(() => {
    if (!busy) return
    const timer = window.setTimeout(() => setSlow(true), showAfterMs)
    return () => {
      window.clearTimeout(timer)
      setSlow(false)
    }
  }, [busy])

  if (!busy || !slow) return null
  return (
    <div
      role="status"
      className="fixed bottom-4 left-1/2 z-40 flex w-[min(460px,calc(100vw-32px))] -translate-x-1/2 animate-rise-in items-start gap-3 bg-purple-light p-4 text-sm shadow-panel"
    >
      <Spinner className="mt-0.5 size-5 shrink-0 text-yellow" />
      <p>
        <strong className="font-extrabold">Waking up the server…</strong> This demo runs on free hosting that
        sleeps when nobody uses it; the first load can take up to a minute.
      </p>
    </div>
  )
}

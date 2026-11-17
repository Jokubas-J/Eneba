import { useEffect, useRef, useState } from 'react'

// Lets a dialog play its closing transition before React removes it.
//
// Our dialogs disappear by being unmounted (the parent stops rendering them), which
// would cut any animation off. requestClose() first sets `closing` (the dialog gets
// data-closing, and the CSS in index.css slides or fades it out), then calls `finish`
// once the transition has had time to run. For a <dialog>, finish should be
// dialog.close(): the browser returns focus to the opener only on close().
export function useAnimatedClose(finish: () => void, durationMs = 250) {
  const [closing, setClosing] = useState(false)
  const timer = useRef<number | undefined>(undefined)

  // If the dialog is removed some other way first, the pending call is dropped.
  useEffect(() => () => window.clearTimeout(timer.current), [])

  function requestClose() {
    if (timer.current !== undefined) return // already closing
    setClosing(true)
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    timer.current = window.setTimeout(finish, reduced ? 0 : durationMs)
  }

  return { closing, requestClose }
}

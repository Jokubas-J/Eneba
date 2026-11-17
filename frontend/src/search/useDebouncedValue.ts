import { useEffect, useState } from 'react'

// Returns value, but only after it has stopped changing for delayMs.
//
// While someone types "witcher", the text changes seven times in about a second.
// Sending a request for each of "w", "wi", "wit"... would waste requests whose
// answers are out of date before they arrive; with a debounce, a request goes out
// when the typing pauses.
export function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value)

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs)
    // The clean-up runs before the effect runs again (that is, on the next change of
    // value): it cancels the pending timer, so only the last value of a burst of
    // changes ever arrives.
    return () => clearTimeout(timer)
  }, [value, delayMs])

  return debounced
}

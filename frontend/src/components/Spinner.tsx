import { LoaderCircle } from 'lucide-react'

// A small turning circle for "working on it" inside buttons and short waits.
// Decorative: the button text next to it says what is happening.
export function Spinner({ className = 'size-4' }: { className?: string }) {
  return <LoaderCircle className={`animate-spin ${className}`} aria-hidden="true" />
}

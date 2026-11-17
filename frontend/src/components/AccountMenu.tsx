import { useEffect, useId, useRef, useState } from 'react'
import { Link } from 'react-router'
import { ChevronIcon, UserIcon } from './icons.tsx'

// The signed-in account in the header: one compact button (icon + e-mail, shortened
// to fit) that opens a small menu with the full address, "My orders" and "Log out".
// Showing all of that inline did not fit next to the search box: with a long address
// the header broke onto three rows (found 2026-11-17).
//
// A disclosure (button + aria-expanded + a list of links), not an ARIA "menu": the
// entries are ordinary links and buttons, reached with Tab like everywhere else.
export function AccountMenu({ email, onLogout }: { email: string; onLogout: () => void }) {
  const [open, setOpen] = useState(false)
  const wrapper = useRef<HTMLDivElement>(null)
  const panelId = useId()

  // Close on a click anywhere else, and on Escape (focus goes back to the button).
  useEffect(() => {
    if (!open) return
    function onPointerDown(event: PointerEvent) {
      if (!wrapper.current?.contains(event.target as Node)) setOpen(false)
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setOpen(false)
        wrapper.current?.querySelector('button')?.focus()
      }
    }
    document.addEventListener('pointerdown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('pointerdown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [open])

  return (
    <div ref={wrapper} className="relative">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        aria-label={`Account: ${email}`}
        onClick={() => setOpen((value) => !value)}
        className="flex items-center gap-2 hover:text-yellow"
      >
        <UserIcon className="size-5 shrink-0" />
        <span className="hidden max-w-[150px] truncate md:inline">{email}</span>
        <ChevronIcon
          direction="right"
          className={`size-3 shrink-0 transition-transform duration-200 ${open ? '-rotate-90' : 'rotate-90'}`}
        />
      </button>
      {open && (
        <div
          id={panelId}
          className="absolute top-full right-0 z-30 mt-3 w-[240px] animate-fade-in bg-purple-light py-2 shadow-panel"
        >
          {/* The full address, wrapped: the button above shows it shortened. */}
          <p className="px-4 pb-2 text-xs break-all text-text-muted">
            {email}
          </p>
          <Link
            to="/orders"
            onClick={() => setOpen(false)}
            className="block px-4 py-2 transition-colors hover:bg-overlay-item hover:text-yellow"
          >
            My orders
          </Link>
          <button
            type="button"
            onClick={() => {
              setOpen(false)
              onLogout()
            }}
            className="block w-full px-4 py-2 text-left transition-colors hover:bg-overlay-item hover:text-yellow"
          >
            Log out
          </button>
        </div>
      )}
    </div>
  )
}

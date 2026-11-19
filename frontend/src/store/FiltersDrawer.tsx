import { useEffect, useRef, type ReactNode } from 'react'
import { Button } from '../components/Button.tsx'
import { CloseIcon } from '../components/icons.tsx'
import { useAnimatedClose } from '../components/useAnimatedClose.ts'

// The store's filters on a phone: a drawer from the left instead of a long sidebar
// above the results (with five filter groups, the first product was a long scroll
// away). The filters inside are the same FilterSidebar as on wide screens and apply
// at once; "Show N results" closes the drawer. Same dialog pattern as the Categories
// drawer: focus stays inside, Escape closes, focus returns to the "Filters" button.
export function FiltersDrawer({
  resultCount,
  onClose,
  children,
}: {
  resultCount: number | null
  onClose: () => void
  children: ReactNode
}) {
  const dialogRef = useRef<HTMLDialogElement>(null)
  const { closing, requestClose } = useAnimatedClose(() => dialogRef.current?.close())

  useEffect(() => {
    const dialog = dialogRef.current
    if (dialog && !dialog.open) dialog.showModal()
  }, [])

  return (
    <dialog
      ref={dialogRef}
      aria-label="Filters"
      data-closing={closing || undefined}
      onClose={onClose}
      onCancel={(event) => {
        event.preventDefault()
        requestClose()
      }}
      onClick={(event) => {
        if (event.target === event.currentTarget) requestClose()
      }}
      className="drawer-dialog m-0 h-dvh max-h-none w-full max-w-[360px] bg-purple-base p-0 text-white shadow-panel"
    >
      <div className="flex h-full flex-col">
        <div className="flex items-center justify-between border-b border-overlay-row px-4 py-3">
          <p className="text-lg font-extrabold">Filters</p>
          <button type="button" onClick={requestClose} aria-label="Close filters" className="p-1 hover:text-yellow">
            <CloseIcon className="size-5" />
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-4">{children}</div>
        <div className="border-t border-overlay-row p-4">
          <Button className="w-full" onClick={requestClose}>
            {resultCount === null ? 'Show results' : `Show ${resultCount} results`}
          </Button>
        </div>
      </div>
    </dialog>
  )
}

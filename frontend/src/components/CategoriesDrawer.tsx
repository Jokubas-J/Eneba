import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'
import type { MenuSection } from '../api.ts'
import { ContentIcon } from './ContentIcon.tsx'
import { ChevronIcon, CloseIcon } from './icons.tsx'

// The "Categories" drawer (docs/gap-analysis.md §2): a panel from the left with the
// menu sections; a group's links appear in a second column on wide screens and open
// in place on phones. Everything listed comes from GET /api/menu.
//
// Built on <dialog> + showModal(), like the other dialogs: the browser keeps focus
// inside, Escape closes it, and focus returns to the "Categories" button afterwards.
export function CategoriesDrawer({
  sections,
  state,
  onRetry,
  onClose,
}: {
  sections: MenuSection[]
  state: 'ready' | 'loading' | 'error'
  onRetry: () => void
  onClose: () => void
}) {
  const dialogRef = useRef<HTMLDialogElement>(null)
  const groups = sections.filter((s) => s.kind !== 'link')
  const [activeId, setActiveId] = useState<number | null>(null)
  // The open group: chosen by tap (phones) or by pointing / tap (wide screens).
  const active = groups.find((g) => g.id === activeId) ?? null

  useEffect(() => {
    // See AddedToCartDialog for why there is no close() in a clean-up.
    const dialog = dialogRef.current
    if (dialog && !dialog.open) dialog.showModal()
  }, [])

  return (
    <dialog
      ref={dialogRef}
      aria-label="Categories"
      onClose={onClose}
      onClick={(event) => {
        // A click on the dimmed backdrop (the dialog element itself) closes it.
        if (event.target === event.currentTarget) onClose()
      }}
      className="m-0 h-dvh max-h-none w-full max-w-[640px] bg-purple-dark p-0 text-white shadow-panel backdrop:bg-black/60"
    >
      <div className="flex h-full flex-col">
        <div className="flex items-center justify-between border-b border-overlay-row px-4 py-3">
          <p className="text-lg font-extrabold">Categories</p>
          <button type="button" onClick={onClose} aria-label="Close categories" className="p-1 hover:text-yellow">
            <CloseIcon className="size-5" />
          </button>
        </div>

        {state === 'error' && (
          <div role="alert" className="p-4">
            <p>The menu could not be loaded.</p>
            <button type="button" onClick={onRetry} className="mt-2 font-semibold text-yellow hover:underline">
              Try again
            </button>
          </div>
        )}
        {state === 'loading' && <div className="m-4 h-[200px] animate-pulse bg-overlay-item" aria-hidden="true" />}

        {state === 'ready' && (
          <div className="flex min-h-0 flex-1">
            <ul className="w-full overflow-y-auto py-2 md:w-[260px] md:shrink-0 md:border-r md:border-overlay-row">
              {sections.map((section) =>
                section.kind === 'link' && section.url ? (
                  <li key={section.id}>
                    <Link to={section.url} onClick={onClose} className={rowClass(false)}>
                      <ContentIcon name={section.icon} className="size-5 shrink-0" />
                      <span className="flex-1">{section.title}</span>
                    </Link>
                  </li>
                ) : (
                  <li key={section.id}>
                    <button
                      type="button"
                      aria-expanded={active?.id === section.id}
                      onClick={() => setActiveId(active?.id === section.id ? null : section.id)}
                      // On wide screens pointing at a group is enough, as on eneba.com.
                      onMouseEnter={() => {
                        if (window.matchMedia('(min-width: 768px)').matches) setActiveId(section.id)
                      }}
                      className={rowClass(active?.id === section.id)}
                    >
                      <ContentIcon name={section.icon} className="size-5 shrink-0" />
                      <span className="flex-1 text-left">{section.title}</span>
                      <ChevronIcon direction="right" className="size-4 shrink-0" />
                    </button>
                    {/* Phones: the group's links open right under it. */}
                    {active?.id === section.id && (
                      <div className="pb-2 pl-6 md:hidden">
                        <LinkList section={section} onNavigate={onClose} />
                      </div>
                    )}
                  </li>
                ),
              )}
            </ul>
            {/* Wide screens: the open group's links in a second column. */}
            <div className="hidden flex-1 overflow-y-auto p-2 md:block">
              {active ? (
                <>
                  <p className="px-2 pt-2 pb-1 text-xs font-extrabold text-text-muted uppercase">{active.title}</p>
                  <LinkList section={active} onNavigate={onClose} />
                </>
              ) : (
                <p className="p-2 text-sm text-text-muted">Point at a group to see what is in it.</p>
              )}
            </div>
          </div>
        )}
      </div>
    </dialog>
  )
}

function rowClass(open: boolean) {
  return `flex w-full items-center gap-3 px-4 py-3 font-semibold hover:bg-overlay-item ${open ? 'bg-overlay-item text-yellow' : ''}`
}

function LinkList({ section, onNavigate }: { section: MenuSection; onNavigate: () => void }) {
  if (section.links.length === 0) return <p className="p-2 text-sm text-text-muted">Nothing here yet.</p>
  return (
    <ul>
      {section.links.map((link) => (
        <li key={link.url}>
          <Link
            to={link.url}
            onClick={onNavigate}
            className="flex items-center gap-3 px-2 py-2 text-sm hover:bg-overlay-item hover:text-yellow"
          >
            <ContentIcon name={link.icon} className="size-4 shrink-0 text-text-muted" />
            <span className="flex-1">{link.label}</span>
            {link.count !== null && <span className="text-xs text-text-muted">{link.count}</span>}
          </Link>
        </li>
      ))}
    </ul>
  )
}

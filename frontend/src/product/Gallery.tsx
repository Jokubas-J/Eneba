import { useEffect, useRef, useState } from 'react'
import type { Screenshot } from '../api.ts'
import { ChevronIcon, CloseIcon } from '../components/icons.tsx'
import { RemoteImage } from '../components/RemoteImage.tsx'

// A row of screenshot thumbnails under the info tiles (176x99, as measured on
// eneba.com). Clicking one opens it full size in a lightbox.
export function Gallery({ screenshots, title }: { screenshots: Screenshot[]; title: string }) {
  // null = lightbox closed; a number = index of the screenshot being shown.
  const [open, setOpen] = useState<number | null>(null)
  // Screenshots whose thumbnail could not be loaded from Steam are left out of the
  // row and of the lightbox, instead of showing a broken image.
  const [broken, setBroken] = useState<ReadonlySet<string>>(new Set())
  const shown = screenshots.filter((shot) => !broken.has(shot.fullUrl))

  if (shown.length === 0) return null

  return (
    <>
      {/* On narrow screens the row scrolls sideways instead of wrapping. */}
      <ul className="flex gap-2 overflow-x-auto pb-1 [scrollbar-color:var(--color-purple-light)_transparent] [scrollbar-width:thin]" aria-label="Screenshots">
        {shown.map((shot, index) => (
          <li key={shot.fullUrl} className="shrink-0">
            <button
              type="button"
              onClick={() => setOpen(index)}
              aria-label={`Open screenshot ${index + 1} of ${shown.length}`}
              className="block h-[99px] w-[176px] bg-purple-dark focus-visible:outline-2 focus-visible:outline-yellow"
            >
              <RemoteImage
                src={shot.thumbnailUrl}
                loading="lazy"
                fallback="hide"
                onFail={() => setBroken((current) => new Set(current).add(shot.fullUrl))}
                className="size-full object-cover hover:opacity-80"
              />
            </button>
          </li>
        ))}
      </ul>
      {open !== null && (
        <Lightbox
          screenshots={shown}
          index={Math.min(open, shown.length - 1)}
          title={title}
          onIndexChange={setOpen}
          onClose={() => setOpen(null)}
        />
      )}
    </>
  )
}

type LightboxProps = {
  screenshots: Screenshot[]
  index: number
  title: string
  onIndexChange: (index: number) => void
  onClose: () => void
}

// The full-size view, built on the browser's own <dialog> element. showModal() puts
// it above everything else, makes the rest of the page inert (keyboard focus cannot
// wander behind it), and closes it on Escape, which a hand-made overlay would all
// have to reimplement.
function Lightbox({ screenshots, index, title, onIndexChange, onClose }: LightboxProps) {
  const dialogRef = useRef<HTMLDialogElement>(null)

  // Open the dialog when the component appears. An effect, because showModal is a
  // method of the real DOM element, which exists only after React has rendered it.
  useEffect(() => {
    const dialog = dialogRef.current
    // No clean-up that calls close(): closing fires the "close" event, which tells
    // the parent to remove the dialog. In development React mounts every component,
    // unmounts it and mounts it again (StrictMode, to expose effects without proper
    // clean-up), and such a clean-up made the dialog close itself at once. Removing
    // the element from the page on unmount is enough to make it disappear.
    if (dialog && !dialog.open) dialog.showModal()
  }, [])

  // Wraps around: "next" on the last screenshot shows the first.
  const go = (step: number) => onIndexChange((index + step + screenshots.length) % screenshots.length)
  const shot = screenshots[index]

  return (
    <dialog
      ref={dialogRef}
      aria-label={`${title}, screenshot ${index + 1} of ${screenshots.length}`}
      // Escape (and the browser's own close) fires "close"; we tell the parent.
      onClose={onClose}
      onKeyDown={(event) => {
        if (event.key === 'ArrowRight') go(1)
        if (event.key === 'ArrowLeft') go(-1)
      }}
      // A click on the dimmed area outside the image lands on the <dialog> itself.
      onClick={(event) => {
        if (event.target === event.currentTarget) onClose()
      }}
      className="m-auto max-h-none max-w-none bg-transparent p-0 backdrop:bg-black/85"
    >
      <div className="relative flex items-center gap-2">
        <button type="button" onClick={() => go(-1)} aria-label="Previous screenshot" className="p-2 text-white">
          <ChevronIcon direction="left" className="size-8" />
        </button>
        <RemoteImage
          src={shot.fullUrl}
          className="max-h-[85vh] max-w-[85vw] object-contain"
          // The full image has no fixed size, so the placeholder needs one.
          placeholderClassName="h-[50vh] w-[min(85vw,900px)]"
        />
        <button type="button" onClick={() => go(1)} aria-label="Next screenshot" className="p-2 text-white">
          <ChevronIcon direction="right" className="size-8" />
        </button>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close"
          // autoFocus: the dialog moves keyboard focus here when it opens.
          autoFocus
          className="absolute -top-10 right-0 p-1 text-white"
        >
          <CloseIcon className="size-7" />
        </button>
      </div>
      <p className="mt-2 text-center text-sm text-text-nav">
        {index + 1} / {screenshots.length}
      </p>
    </dialog>
  )
}

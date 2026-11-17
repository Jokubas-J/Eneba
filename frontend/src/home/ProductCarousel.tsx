import { type MouseEvent, type PointerEvent, useEffect, useRef, useState } from 'react'
import type { ProductCard as Product } from '../api.ts'
import { ChevronIcon } from '../components/icons.tsx'
import { ProductCard } from '../components/ProductCard.tsx'

// A row of product cards that scrolls sideways (screenshot 04): arrows on the sides,
// a thin progress bar under it.
//
// The scrolling itself is the browser's (overflow-x + scroll-snap), not a carousel
// library: touch, trackpad and keyboard scrolling work for free. On top of that:
// the arrows call scrollBy, and a mouse can drag the row (below).
export function ProductCarousel({ items }: { items: Product[] }) {
  const scroller = useRef<HTMLUListElement>(null)
  // Which part of the row is visible, as fractions of its full width.
  const [view, setView] = useState({ start: 0, size: 1 })
  // Mouse dragging: where the press started, and whether it has moved far enough to
  // count as a drag (then the click that ends it must not open a product).
  const drag = useRef<{ x: number; scrollLeft: number; moved: boolean } | null>(null)
  const [dragging, setDragging] = useState(false)

  function measure() {
    const el = scroller.current
    if (!el || el.scrollWidth === 0) return
    setView({ start: el.scrollLeft / el.scrollWidth, size: el.clientWidth / el.scrollWidth })
  }

  useEffect(() => {
    measure()
    // The visible fraction changes when the window is resized.
    window.addEventListener('resize', measure)
    return () => window.removeEventListener('resize', measure)
  }, [items])

  function scrollByPage(direction: 1 | -1) {
    const el = scroller.current
    if (el) el.scrollBy({ left: direction * el.clientWidth * 0.9, behavior: 'smooth' })
  }

  // Touch and pen already scroll natively; only the mouse needs drag support.
  function onPointerDown(event: PointerEvent<HTMLUListElement>) {
    if (event.pointerType !== 'mouse' || event.button !== 0 || !scroller.current) return
    drag.current = { x: event.clientX, scrollLeft: scroller.current.scrollLeft, moved: false }
  }

  function onPointerMove(event: PointerEvent<HTMLUListElement>) {
    const el = scroller.current
    const start = drag.current
    if (!el || !start) return
    const dx = event.clientX - start.x
    if (!start.moved && Math.abs(dx) < 5) return // a small wobble is still a click
    if (!start.moved) {
      start.moved = true
      setDragging(true)
      // Keep receiving the moves even when the pointer leaves the row.
      el.setPointerCapture(event.pointerId)
    }
    el.scrollLeft = start.scrollLeft - dx
  }

  function endDrag() {
    if (!drag.current) return
    const moved = drag.current.moved
    setDragging(false)
    // Cleared after this event, so the click that follows can still see `moved`.
    window.setTimeout(() => {
      drag.current = null
    })
    if (moved) {
      // Snapping was off during the drag; let the row settle on the nearest card.
      scroller.current?.scrollBy({ left: 0, behavior: 'smooth' })
    }
  }

  // Runs before the card link's own click (capture phase): a drag is not a click.
  function onClickCapture(event: MouseEvent) {
    if (drag.current?.moved) {
      event.preventDefault()
      event.stopPropagation()
    }
  }

  const atStart = view.start <= 0.001
  const atEnd = view.start + view.size >= 0.999

  return (
    <div className="relative">
      <ul
        ref={scroller}
        onScroll={measure}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={endDrag}
        onPointerCancel={endDrag}
        onClickCapture={onClickCapture}
        // Browsers start dragging an image or link "ghost" on mouse-down-and-move;
        // that would steal the gesture from the row.
        onDragStart={(event) => event.preventDefault()}
        className={`flex gap-4 overflow-x-auto pb-1 [scrollbar-width:none] ${
          dragging ? 'cursor-grabbing snap-none select-none' : 'snap-x snap-mandatory md:cursor-grab'
        }`}
      >
        {items.map((product) => (
          <li key={product.id} className="w-[195px] shrink-0 snap-start">
            <ProductCard product={product} />
          </li>
        ))}
      </ul>

      <Arrow direction="left" hidden={atStart} onClick={() => scrollByPage(-1)} />
      <Arrow direction="right" hidden={atEnd} onClick={() => scrollByPage(1)} />

      {view.size < 1 && (
        <div className="mt-4 h-1 bg-purple-light" aria-hidden="true">
          <div
            className="h-full bg-white transition-[margin] duration-150"
            style={{ width: `${view.size * 100}%`, marginLeft: `${view.start * 100}%` }}
          />
        </div>
      )}
    </div>
  )
}

function Arrow({ direction, hidden, onClick }: { direction: 'left' | 'right'; hidden: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={direction === 'left' ? 'Scroll left' : 'Scroll right'}
      // Fades out at the ends instead of popping away; hidden arrows cannot be
      // clicked or reached with Tab. Not shown on touch-sized screens, where swiping
      // is the natural way.
      tabIndex={hidden ? -1 : undefined}
      aria-hidden={hidden || undefined}
      className={`absolute top-[120px] hidden size-9 items-center justify-center bg-black/50 text-white transition-opacity duration-200 hover:bg-black/70 md:flex ${
        direction === 'left' ? 'left-0' : 'right-0'
      } ${hidden ? 'pointer-events-none opacity-0' : 'opacity-100'}`}
    >
      <ChevronIcon direction={direction} className="size-5" />
    </button>
  )
}

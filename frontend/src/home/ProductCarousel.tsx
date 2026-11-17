import { useEffect, useRef, useState } from 'react'
import type { ProductCard as Product } from '../api.ts'
import { ChevronIcon } from '../components/icons.tsx'
import { ProductCard } from '../components/ProductCard.tsx'

// A row of product cards that scrolls sideways (screenshot 04): arrows on the sides,
// a thin progress bar under it.
//
// The scrolling itself is the browser's (overflow-x + scroll-snap), not a carousel
// library: touch, trackpad and keyboard scrolling then work for free, and the
// arrows only call scrollBy. No new dependency for what CSS already does.
export function ProductCarousel({ items }: { items: Product[] }) {
  const scroller = useRef<HTMLUListElement>(null)
  // Which part of the row is visible, as fractions of its full width.
  const [view, setView] = useState({ start: 0, size: 1 })

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

  const atStart = view.start <= 0.001
  const atEnd = view.start + view.size >= 0.999

  return (
    <div className="relative">
      <ul
        ref={scroller}
        onScroll={measure}
        className="flex snap-x snap-mandatory gap-4 overflow-x-auto pb-1 [scrollbar-width:none]"
      >
        {items.map((product) => (
          <li key={product.id} className="w-[195px] shrink-0 snap-start">
            <ProductCard product={product} />
          </li>
        ))}
      </ul>

      {!atStart && <Arrow direction="left" onClick={() => scrollByPage(-1)} />}
      {!atEnd && <Arrow direction="right" onClick={() => scrollByPage(1)} />}

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

function Arrow({ direction, onClick }: { direction: 'left' | 'right'; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={direction === 'left' ? 'Scroll left' : 'Scroll right'}
      // Hidden on touch-sized screens, where swiping is the natural way.
      className={`absolute top-[120px] hidden size-9 items-center justify-center bg-black/50 text-white hover:bg-black/70 md:flex ${
        direction === 'left' ? 'left-0' : 'right-0'
      }`}
    >
      <ChevronIcon direction={direction} className="size-5" />
    </button>
  )
}

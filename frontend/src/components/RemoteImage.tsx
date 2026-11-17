import { useState } from 'react'

type Props = {
  src: string
  className?: string
  loading?: 'lazy' | 'eager'
  // What to show when the image cannot be loaded:
  //   'placeholder' - a neutral box with an icon, keeping the layout (covers);
  //   'hide'        - nothing at all (an optional extra such as a screenshot).
  fallback?: 'placeholder' | 'hide'
  // Called once when loading fails, so a parent can drop the item (gallery).
  onFail?: () => void
  // Classes for the placeholder instead of className, when the image itself has no
  // fixed size to inherit (the lightbox).
  placeholderClassName?: string
}

// An image hot-linked from Steam's CDN. Covers and screenshots are not stored by us:
// only their URLs are. If Steam removes or moves an image, the browser would show its
// broken-image icon; this component shows a placeholder or nothing instead.
//
// The <img> element's "error" event fires when the image cannot be loaded (404,
// network error, not an image). An empty URL is treated the same way without
// requesting it.
//
// Decorative (alt=""): every image here sits next to the product name, so a screen
// reader would only repeat it.
export function RemoteImage({ src, className, loading, fallback = 'placeholder', onFail, placeholderClassName }: Props) {
  // Which URL failed. Comparing with the current src (instead of a plain true/false)
  // means a new URL - another product in a reused component - is tried again.
  const [failedSrc, setFailedSrc] = useState<string | null>(null)
  const failed = src === '' || failedSrc === src

  if (failed) {
    if (fallback === 'hide') return null
    return (
      <span className={`flex items-center justify-center bg-purple-dark text-text-muted ${placeholderClassName ?? className ?? ''}`}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.5} className="size-1/3 max-w-12" aria-hidden="true">
          <rect x="3" y="5" width="18" height="14" />
          <circle cx="9" cy="10" r="1.5" />
          <path d="m21 16-5-5-8 8" />
        </svg>
      </span>
    )
  }

  return (
    <img
      src={src}
      alt=""
      loading={loading}
      className={className}
      onError={() => {
        setFailedSrc(src)
        onFail?.()
      }}
    />
  )
}

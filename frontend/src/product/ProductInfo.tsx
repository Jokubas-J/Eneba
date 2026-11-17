import { type KeyboardEvent, type ReactNode, useLayoutEffect, useRef, useState } from 'react'
import { Link } from 'react-router'
import type { ActivationGuide, ProductDetail } from '../api.ts'
import { InfoIcon } from '../components/icons.tsx'
import { formatDate } from './formatDate.ts'

// The yellow-bordered box under the gallery (design-reference.md §5): what the
// visitor must know before buying a key for this platform. The text comes from
// the backend (activation_guides table).
export function ImportantNotice({ guide }: { guide: ActivationGuide }) {
  return (
    <aside aria-label="Important notice" className="flex gap-4 border border-yellow bg-purple-dark px-6 py-4">
      <InfoIcon className="mt-0.5 size-6 shrink-0 text-yellow" />
      <div className="text-[13px] leading-[20px]">
        <p className="text-[15px] font-extrabold text-yellow">Important notice:</p>
        <p>{guide.notice}</p>
      </div>
    </aside>
  )
}

type Tab = 'description' | 'activation'

// "Product details": genre tags, then two tabs (Description / Activation guide), then
// the details table, in the order eneba.com uses.
export function ProductInfo({ product }: { product: ProductDetail }) {
  const [tab, setTab] = useState<Tab>('description')
  const tabs: { id: Tab; label: string }[] = [
    { id: 'description', label: 'Description' },
    ...(product.activation ? [{ id: 'activation' as const, label: 'Activation guide' }] : []),
  ]

  // The ARIA "tabs" pattern: only the selected tab is in the Tab-key order, and the
  // arrow keys move between tabs (that is what screen-reader users expect).
  function handleKeyDown(event: KeyboardEvent) {
    if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return
    const current = tabs.findIndex((t) => t.id === tab)
    const next = tabs[(current + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length]
    setTab(next.id)
    document.getElementById(`tab-${next.id}`)?.focus()
  }

  return (
    <section aria-labelledby="details-heading">
      <h2 id="details-heading" className="mb-4 text-xl font-bold">
        Product details
      </h2>

      {product.genres.length > 0 && (
        <ul className="mb-6 flex flex-wrap gap-1" aria-label="Genres">
          {product.genres.map((genre) => (
            <li key={genre.slug}>
              <Link
                to={`/store?genre=${genre.slug}`}
                className="block bg-overlay-row px-2 text-xs leading-[23px] font-semibold text-text-nav hover:text-white"
              >
                {genre.name}
              </Link>
            </li>
          ))}
        </ul>
      )}

      <div className="bg-overlay-row">
        <div role="tablist" aria-label="Product information" className="flex border-b border-purple-light">
          {tabs.map((t) => (
            <button
              key={t.id}
              id={`tab-${t.id}`}
              type="button"
              role="tab"
              aria-selected={tab === t.id}
              aria-controls={`panel-${t.id}`}
              tabIndex={tab === t.id ? 0 : -1}
              onClick={() => setTab(t.id)}
              onKeyDown={handleKeyDown}
              className={`h-10 px-4 text-sm font-extrabold ${
                tab === t.id ? 'border-b-2 border-yellow text-white' : 'text-text-muted hover:text-white'
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>

        <div key={tab} id={`panel-${tab}`} role="tabpanel" aria-labelledby={`tab-${tab}`} className="animate-fade-in p-4">
          {tab === 'description' ? (
            <ReadMore>
              <h3 className="mb-3 font-extrabold">{product.name}</h3>
              {/* Plain text from Steam's short description; React escapes it, so no
                  HTML from the data can run in the page. */}
              <p className="text-sm leading-[24px]">{product.description || 'No description available.'}</p>
            </ReadMore>
          ) : (
            product.activation && <ActivationSteps guide={product.activation} />
          )}
        </div>
      </div>

      <h2 className="mt-8 mb-4 text-xl font-bold">Other details</h2>
      <dl className="grid grid-cols-[max-content_1fr] gap-x-6 gap-y-1 bg-overlay-row p-4 text-sm">
        <Detail label="Platform">{product.platform.name}</Detail>
        <Detail label="Region">{product.region.name}</Detail>
        {product.releaseDate && <Detail label="Release date">{formatDate(product.releaseDate)}</Detail>}
        {product.developer && <Detail label="Developer">{product.developer}</Detail>}
        {product.publisher && <Detail label="Publisher">{product.publisher}</Detail>}
      </dl>
      <p className="mt-3 text-xs text-text-muted">
        Title, description, genres, dates, cover and screenshots from the Steam store. Sellers, prices and stock are
        demo data.
      </p>
    </section>
  )
}

function ActivationSteps({ guide }: { guide: ActivationGuide }) {
  return (
    <div className="text-sm leading-[24px]">
      <ol className="list-decimal space-y-1 pl-6">
        {guide.steps.map((step) => (
          <li key={step}>{step}</li>
        ))}
      </ol>
      {guide.redeemUrl && (
        <p className="mt-3">
          Activation page:{' '}
          {/* An external site: a plain <a>; noopener so the new tab cannot reach back
              into this page through window.opener. */}
          <a href={guide.redeemUrl} target="_blank" rel="noopener noreferrer" className="text-yellow underline">
            {new URL(guide.redeemUrl).hostname}
          </a>
        </p>
      )}
      <p className="mt-3 text-xs text-text-muted">
        This is a demo shop: the keys it sells are not real and cannot be activated.
      </p>
    </div>
  )
}

// The height at which long text is cut, with a "Read more" button to expand it.
const collapsedHeight = 200

// Shows its content cut to collapsedHeight with a "Read more" button, but only when
// the content is actually taller than that. Whether it is can only be known after
// the browser has laid it out, so it is measured in useLayoutEffect (which runs
// after layout but before the screen is painted, so there is no visible jump).
function ReadMore({ children }: { children: ReactNode }) {
  const contentRef = useRef<HTMLDivElement>(null)
  const [tooTall, setTooTall] = useState(false)
  const [expanded, setExpanded] = useState(false)

  useLayoutEffect(() => {
    const element = contentRef.current
    if (element) setTooTall(element.scrollHeight > collapsedHeight)
  }, [children])

  const collapsed = tooTall && !expanded

  return (
    <div>
      <div
        ref={contentRef}
        style={collapsed ? { maxHeight: collapsedHeight } : undefined}
        className={collapsed ? 'overflow-hidden [mask-image:linear-gradient(black_60%,transparent)]' : ''}
      >
        {children}
      </div>
      {tooTall && (
        <button
          type="button"
          onClick={() => setExpanded(!expanded)}
          aria-expanded={expanded}
          className="mx-auto mt-2 block text-sm font-semibold text-yellow hover:underline"
        >
          {expanded ? 'Read less' : 'Read more'}
        </button>
      )}
    </div>
  )
}

function Detail({ label, children }: { label: string; children: ReactNode }) {
  return (
    <>
      <dt className="text-text-muted">{label}</dt>
      <dd>{children}</dd>
    </>
  )
}

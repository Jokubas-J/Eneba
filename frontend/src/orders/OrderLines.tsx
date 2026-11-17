import { useState } from 'react'
import { Link } from 'react-router'
import type { OrderItem } from '../api.ts'
import { RemoteImage } from '../components/RemoteImage.tsx'
import { formatPrice } from '../money.ts'

// The lines of an order, styled like the cart's rows. showKeys adds each line's
// delivered keys with a copy button (paid orders only; the server sends no keys
// before payment anyway).
export function OrderLines({ items, showKeys }: { items: OrderItem[]; showKeys: boolean }) {
  return (
    <ul className="flex flex-col gap-2">
      {items.map((item) => (
        <li key={item.id} className="flex gap-4 bg-overlay-item p-4">
          <Link to={`/product/${item.productSlug}`} className="h-[84px] w-[60px] shrink-0 bg-purple-deep">
            <RemoteImage src={item.coverUrl} className="size-full object-cover" />
          </Link>
          <div className="min-w-0 flex-1">
            <p className="text-xs text-text-muted">
              Sold by <span className="font-semibold text-white">{item.sellerName}</span>
            </p>
            <Link to={`/product/${item.productSlug}`} className="mt-1 block text-sm font-extrabold hover:underline">
              {item.productName}
            </Link>
            <p className="mt-1 text-xs text-text-muted">
              {item.quantity} × {formatPrice(item.unitPriceCents)}
            </p>
            {showKeys && (
              <ul className="mt-3 flex flex-col gap-2" aria-label={`Keys for ${item.productName}`}>
                {item.keys.map((key) => (
                  <KeyRow key={key} value={key} />
                ))}
              </ul>
            )}
          </div>
          <p className="shrink-0 font-extrabold">{formatPrice(item.unitPriceCents * item.quantity)}</p>
        </li>
      ))}
    </ul>
  )
}

function KeyRow({ value }: { value: string }) {
  const [copied, setCopied] = useState(false)

  async function copy() {
    // The Clipboard API works only on https (or localhost); without it the key is
    // still selectable by hand.
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      setCopied(false)
    }
  }

  return (
    <li className="flex flex-wrap items-center gap-2">
      {/* select-all: one click selects the whole key, for copying by hand. */}
      <code className="border border-teal bg-purple-deep px-3 py-1 font-mono text-sm tracking-wider text-teal select-all">
        {value}
      </code>
      <button type="button" onClick={copy} className="text-xs font-semibold text-yellow hover:underline">
        {copied ? 'Copied' : 'Copy'}
      </button>
      <span aria-live="polite" className="sr-only">
        {copied ? 'Key copied' : ''}
      </span>
    </li>
  )
}

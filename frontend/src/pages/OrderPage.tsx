import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { useParams, useSearchParams } from 'react-router'
import { ApiError, confirmOrder, fetchOrder, type Order } from '../api.ts'
import { useCart } from '../cart/useCart.ts'
import { Button, ButtonLink } from '../components/Button.tsx'
import { InfoIcon, VerifiedIcon } from '../components/icons.tsx'
import { Page } from '../components/Page.tsx'
import { formatPrice } from '../money.ts'
import { OrderLines } from '../orders/OrderLines.tsx'
import { RequireUser } from '../orders/RequireUser.tsx'

// How long the page keeps asking for the payment result after returning from Stripe.
const pollEveryMs = 2000
const giveUpAfterMs = 60_000

export function OrderPage() {
  return (
    <RequireUser>
      <OrderView />
    </RequireUser>
  )
}

// Step 3 of the checkout ("Get your product"): the order and, once it is paid, its
// keys. Stripe sends the buyer here with ?session_id=… right after paying. The webhook
// that marks the order paid may arrive a moment later, so while the order is pending
// the page calls /confirm (which asks Stripe directly) every 2 seconds for a minute.
function OrderView() {
  const { id = '' } = useParams()
  const [searchParams] = useSearchParams()
  const returningFromPayment = searchParams.has('session_id')
  const { lines, dispatch } = useCart()

  const order = useQuery({
    queryKey: ['order', id],
    // Coming back from Stripe: ask the server to check the payment now. Otherwise a
    // plain read is enough.
    queryFn: () => (returningFromPayment ? confirmOrder(id) : fetchOrder(id)),
    retry: (count, error) => !(error instanceof ApiError && error.status < 500) && count < 3,
    // Poll only while waiting for a payment that should be about to arrive.
    refetchInterval: (query) => {
      const data = query.state.data
      const waited = Date.now() - query.state.dataUpdatedAt
      if (!returningFromPayment || data?.status !== 'pending' || waited > giveUpAfterMs) return false
      return pollEveryMs
    },
  })

  // Once paid, the bought offers leave the cart. Done here and not at checkout, so a
  // buyer who goes back from the payment page still finds their cart.
  const paidOfferIds = order.data?.status === 'paid' ? order.data.items.map((item) => item.offerId) : []
  useEffect(() => {
    for (const offerId of paidOfferIds) {
      if (lines.some((line) => line.offerId === offerId)) dispatch({ type: 'remove', offerId })
    }
    // Runs when the set of paid offers changes, not on every cart change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [paidOfferIds.join(',')])

  if (order.error instanceof ApiError && order.error.status === 404) {
    return (
      <Page>
        <Message title="Order not found" text="This order does not exist or belongs to another account." />
      </Page>
    )
  }
  if (order.isError) {
    return (
      <Page>
        <div className="bg-purple-dark p-8 text-center">
          <p>The order could not be loaded.</p>
          <Button className="mt-3" onClick={() => order.refetch()}>
            Try again
          </Button>
        </div>
      </Page>
    )
  }
  if (!order.data) {
    return (
      <Page>
        <div className="h-[200px] animate-pulse bg-purple-dark" aria-hidden="true" />
      </Page>
    )
  }
  return (
    <Page>
      <OrderDetails order={order.data} waiting={returningFromPayment} />
    </Page>
  )
}

function OrderDetails({ order, waiting }: { order: Order; waiting: boolean }) {
  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-4 lg:grid-cols-[minmax(0,1fr)_380px] lg:items-start">
      <section aria-labelledby="order-heading" className="bg-purple-dark p-4">
        <StatusBanner order={order} waiting={waiting} />
        <h1 id="order-heading" className="mt-4 mb-4 text-lg font-extrabold">
          Order #{order.id}
        </h1>
        <OrderLines items={order.items} showKeys={order.status === 'paid'} />
      </section>

      <aside aria-labelledby="order-summary" className="bg-purple-dark p-4">
        <h2 id="order-summary" className="text-lg font-extrabold">
          Summary
        </h2>
        <dl className="mt-4 grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-sm">
          <dt className="text-text-muted">Placed</dt>
          <dd>{new Date(order.createdAt).toLocaleString('en-IE')}</dd>
          <dt className="text-text-muted">Payment</dt>
          <dd>{order.paymentProvider === 'stripe' ? 'Card (Stripe, test mode)' : 'Simulated payment'}</dd>
          {order.paidAt && (
            <>
              <dt className="text-text-muted">Paid</dt>
              <dd>{new Date(order.paidAt).toLocaleString('en-IE')}</dd>
            </>
          )}
        </dl>
        <dl className="mt-4 flex items-baseline justify-between border-t border-overlay-row pt-4">
          <dt className="font-extrabold">Total:</dt>
          <dd className="text-buybox font-extrabold">{formatPrice(order.totalCents)}</dd>
        </dl>
        <ButtonLink to="/orders" variant="outline" className="mt-4 w-full">
          All my orders
        </ButtonLink>
      </aside>
    </div>
  )
}

function StatusBanner({ order, waiting }: { order: Order; waiting: boolean }) {
  if (order.status === 'paid') {
    return (
      <div className="flex items-center gap-3 border border-teal p-4">
        <VerifiedIcon className="size-8 shrink-0 text-teal" />
        <div>
          <p className="font-extrabold">Payment received. Your keys are below.</p>
          <p className="text-xs text-text-muted">
            Demo shop: these keys are generated and cannot be activated on a real platform.
          </p>
        </div>
      </div>
    )
  }
  if (order.status === 'expired') {
    return (
      <div className="border border-pink-region p-4">
        <p className="font-extrabold text-pink-region">This order expired before it was paid.</p>
        <p className="mt-1 text-xs text-text-muted">The keys were released. You can check out again from your cart.</p>
        <ButtonLink to="/cart" className="mt-3">
          Back to cart
        </ButtonLink>
      </div>
    )
  }
  return (
    <div className="flex items-center gap-3 border border-yellow p-4" role="status">
      <InfoIcon className="size-8 shrink-0 text-yellow" />
      <div>
        <p className="font-extrabold">{waiting ? 'Confirming your payment…' : 'Waiting for payment'}</p>
        <p className="text-xs text-text-muted">
          Your keys are reserved until {new Date(order.expiresAt).toLocaleTimeString('en-IE')}.
        </p>
      </div>
    </div>
  )
}

function Message({ title, text }: { title: string; text: string }) {
  return (
    <div className="flex flex-col items-center py-8 text-center">
      <InfoIcon className="size-8 text-text-nav" />
      <h1 className="mt-4 text-xl font-extrabold">{title}</h1>
      <p className="mt-1 text-xs text-text-muted">{text}</p>
      <ButtonLink to="/orders" className="mt-4">
        My orders
      </ButtonLink>
    </div>
  )
}

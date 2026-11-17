import { useMutation, useQuery } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router'
import { ApiError, fetchOrder, paySimulated } from '../api.ts'
import { Button, ButtonLink } from '../components/Button.tsx'
import { Page } from '../components/Page.tsx'
import { formatPrice } from '../money.ts'
import { RequireUser } from '../orders/RequireUser.tsx'

export function SimulatedPaymentPage() {
  return (
    <RequireUser>
      <SimulatedPayment />
    </RequireUser>
  )
}

// Step 2 when PAYMENT_PROVIDER=simulated: stands in for Stripe's payment page. No card
// is asked for and no money moves; "Pay" tells our backend to treat the order as paid
// (POST /api/payments/simulated/{id}), which runs the same fulfilment as Stripe's
// webhook.
function SimulatedPayment() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const order = useQuery({ queryKey: ['order', id], queryFn: () => fetchOrder(id) })
  const pay = useMutation({
    mutationFn: () => paySimulated(id),
    onSuccess: () => navigate(`/orders/${id}`, { replace: true }),
  })

  const data = order.data
  return (
    <Page>
      <div className="mx-auto max-w-[480px] bg-purple-dark p-6">
        <p className="text-tiny font-extrabold text-yellow uppercase">Simulated payment</p>
        <h1 className="mt-2 text-xl font-extrabold">Pay for order #{id}</h1>
        <p className="mt-2 text-sm text-text-muted">
          This demo is running without Stripe. Nothing is charged; the button below completes the payment.
        </p>

        {order.isError && <p className="mt-4 text-pink-region">The order could not be loaded.</p>}
        {data && (
          <>
            <ul className="mt-4 flex flex-col gap-1 text-sm">
              {data.items.map((item) => (
                <li key={item.id} className="flex justify-between gap-4">
                  <span className="min-w-0 truncate">
                    {item.quantity} × {item.productName}
                  </span>
                  <span className="shrink-0">{formatPrice(item.unitPriceCents * item.quantity)}</span>
                </li>
              ))}
            </ul>
            <p className="mt-4 flex items-baseline justify-between border-t border-overlay-row pt-4">
              <span className="font-extrabold">Total:</span>
              <span className="text-buybox font-extrabold">{formatPrice(data.totalCents)}</span>
            </p>
            {data.status === 'pending' ? (
              <Button className="mt-6 w-full" loading={pay.isPending} onClick={() => pay.mutate()}>
                {pay.isPending ? 'Paying…' : `Pay ${formatPrice(data.totalCents)}`}
              </Button>
            ) : (
              <ButtonLink to={`/orders/${id}`} className="mt-6 w-full">
                View order
              </ButtonLink>
            )}
            {pay.error && (
              <p role="alert" className="mt-3 text-sm text-pink-region">
                {pay.error instanceof ApiError ? pay.error.message : 'The payment failed. Please try again.'}
              </p>
            )}
            <ButtonLink to="/cart" variant="outline" className="mt-3 w-full">
              Cancel and return to cart
            </ButtonLink>
          </>
        )}
      </div>
    </Page>
  )
}

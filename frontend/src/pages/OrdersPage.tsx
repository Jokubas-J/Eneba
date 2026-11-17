import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import { fetchOrders, type OrderStatus } from '../api.ts'
import { Button, ButtonLink } from '../components/Button.tsx'
import { InfoIcon } from '../components/icons.tsx'
import { Page } from '../components/Page.tsx'
import { formatPrice } from '../money.ts'
import { RequireUser } from '../orders/RequireUser.tsx'

export function OrdersPage() {
  return (
    <RequireUser>
      <OrdersList />
    </RequireUser>
  )
}

const statusStyle: Record<OrderStatus, { label: string; className: string }> = {
  paid: { label: 'Paid', className: 'border-teal text-teal' },
  pending: { label: 'Waiting for payment', className: 'border-yellow text-yellow' },
  expired: { label: 'Expired', className: 'border-pink-region text-pink-region' },
}

// Order history: every order of the signed-in user, newest first.
function OrdersList() {
  const orders = useQuery({ queryKey: ['orders'], queryFn: fetchOrders })

  return (
    <Page>
      <h1 className="text-heading font-bold">My orders</h1>
      <div className="mt-6">
        {orders.isError ? (
          <div className="bg-purple-dark p-8 text-center">
            <p>Your orders could not be loaded.</p>
            <Button className="mt-3" onClick={() => orders.refetch()}>
              Try again
            </Button>
          </div>
        ) : !orders.data ? (
          <div className="h-[120px] animate-pulse bg-purple-dark" aria-hidden="true" />
        ) : orders.data.length === 0 ? (
          <div className="flex flex-col items-center bg-purple-dark py-8 text-center">
            <InfoIcon className="size-8 text-text-nav" />
            <p className="mt-4 text-xl font-extrabold">No orders yet</p>
            <ButtonLink to="/store" className="mt-4">
              Browse the store
            </ButtonLink>
          </div>
        ) : (
          <ul className="flex flex-col gap-2">
            {orders.data.map((order) => {
              const status = statusStyle[order.status]
              const keys = order.items.reduce((sum, item) => sum + item.quantity, 0)
              return (
                <li key={order.id}>
                  <Link
                    to={`/orders/${order.id}`}
                    className="flex flex-wrap items-center gap-4 bg-overlay-row p-4 hover:bg-purple-dark"
                  >
                    <div className="min-w-0 flex-1">
                      <p className="font-extrabold">
                        {/* A real space, not only margin: otherwise the link's
                            accessible name runs together ("Order #117/11/2026"). */}
                        Order #{order.id}{' '}
                        <span className="ml-1 text-xs font-normal text-text-muted">
                          {new Date(order.createdAt).toLocaleDateString('en-IE')}
                        </span>
                      </p>
                      <p className="mt-1 truncate text-sm text-text-nav">
                        {order.items.map((item) => item.productName).join(', ')}
                      </p>
                    </div>
                    <span className={`rounded-pill border px-2 text-xs leading-[20px] font-extrabold ${status.className}`}>
                      {status.label}
                    </span>
                    <span className="w-[90px] text-right text-sm text-text-muted">
                      {keys} {keys === 1 ? 'key' : 'keys'}
                    </span>
                    <span className="w-[90px] text-right font-extrabold">{formatPrice(order.totalCents)}</span>
                  </Link>
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </Page>
  )
}

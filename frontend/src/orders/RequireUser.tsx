import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router'
import { useCurrentUser } from '../auth/useAuth.ts'

// Shows its children only to a signed-in visitor. Anyone else is sent to the login
// page with ?next= set to the current address, so signing in brings them back here.
// (The real protection is on the server, which answers 401; this only avoids
// showing a page that cannot load.)
export function RequireUser({ children }: { children: ReactNode }) {
  const me = useCurrentUser()
  const location = useLocation()
  if (me.isPending) return null
  if (!me.data) {
    const next = encodeURIComponent(location.pathname + location.search)
    return <Navigate to={`/login?next=${next}`} replace />
  }
  return <>{children}</>
}

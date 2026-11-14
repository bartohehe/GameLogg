import { Navigate, useLocation } from 'react-router'
import { AppLayout } from '../layout/AppLayout'
import { isUnauthorized } from '../lib/api'
import { useConfig, useMe } from '../lib/session'
import { ErrorScreen } from './ErrorScreen'

/** Wpuszcza do aplikacji tylko z sesją. Bez AUTH_ENABLED backend zawsze zwraca konto owner. */
export function RequireUser() {
  const me = useMe()
  const location = useLocation()

  if (me.isPending) return null
  if (me.isError) {
    if (isUnauthorized(me.error)) {
      return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />
    }
    return <ErrorScreen message={me.error.message} onRetry={() => me.refetch()} />
  }
  return <AppLayout user={me.data} />
}

/** Ekrany logowania istnieją tylko przy AUTH_ENABLED=true. */
export function AuthOnly({ children }: { children: React.ReactNode }) {
  const config = useConfig()
  if (!config.authEnabled) return <Navigate to="/" replace />
  return children
}

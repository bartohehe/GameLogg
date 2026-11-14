import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { useNavigate } from 'react-router'
import { api } from '../lib/api'
import { monthYearGenitive } from '../lib/format'
import { meKey, useConfig, useMe } from '../lib/session'

// Wersja z etapu 1: konto i wylogowanie. Statystyki, „Najwyższa półka”,
// zmiana hasła i „Ten serwer” dochodzą w etapie 6.
export function ProfilePage() {
  const config = useConfig()
  const me = useMe()
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  useEffect(() => {
    document.title = 'Profil · GameLogg'
  }, [])

  const logout = useMutation({
    mutationFn: api.logout,
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: meKey })
      navigate('/login', { replace: true })
    },
  })

  const user = me.data
  if (!user) return null

  return (
    <div className="page">
      <h1 className="page-title">{user.username}</h1>
      <p className="page-lead">
        @{user.username}, na półkach od {monthYearGenitive(new Date(user.createdAt))}
      </p>

      {config.authEnabled && (
        <section aria-labelledby="account" style={{ marginTop: 44 }}>
          <h2 id="account" style={{ margin: '0 0 12px', fontSize: 20, fontWeight: 800, fontStretch: '110%' }}>
            Konto
          </h2>
          <button
            type="button"
            className="btn btn-ghost"
            aria-busy={logout.isPending}
            onClick={() => logout.mutate()}
          >
            Wyloguj się
          </button>
          {logout.isError && (
            <p role="alert" style={{ color: 'var(--warn)' }}>
              {logout.error.message}
            </p>
          )}
        </section>
      )}
    </div>
  )
}

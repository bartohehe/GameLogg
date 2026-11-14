import { useEffect, useState } from 'react'
import { ApiError, getHealth, type Health } from './lib/api'

type State = { kind: 'loading' } | { kind: 'ok'; health: Health } | { kind: 'error'; message: string }

// Ekran tymczasowy z etapu 0: potwierdza, że frontend, API i baza się widzą.
// W etapie 1 zastąpi go layout z nawigacją.
export default function App() {
  const [state, setState] = useState<State>({ kind: 'loading' })

  useEffect(() => {
    let alive = true
    getHealth()
      .then((health) => alive && setState({ kind: 'ok', health }))
      .catch((err: unknown) => {
        if (!alive) return
        const message = err instanceof ApiError ? err.message : 'Nieznany błąd.'
        setState({ kind: 'error', message })
      })
    return () => {
      alive = false
    }
  }, [])

  return (
    <main className="boot">
      <h1 className="wordmark">gamelogg</h1>
      <p role="status" className="boot-status">
        {state.kind === 'loading' && 'Łączę z serwerem…'}
        {state.kind === 'ok' && `Serwer działa, baza odpowiada. Wersja ${state.health.version}.`}
        {state.kind === 'error' && state.message}
      </p>
    </main>
  )
}

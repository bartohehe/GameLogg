import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router'
import { ApiError, api, type User } from '../lib/api'
import { meKey, useMe } from '../lib/session'
import styles from './LoginPage.module.css'

type Mode = 'login' | 'register'
type Field = 'username' | 'email' | 'password'

// Dekoracyjna półka po lewej: kolory i wysokości jak w mockupie
// (200 px PlayStation/Xbox, 186 px PC, 166 px Switch).
const SHELF: Array<[string, string, string, number]> = [
  ['#2A2140', '#C9A04B', 'var(--st-completed)', 200],
  ['#2E2A22', '#D7B55B', 'var(--st-completed)', 186],
  ['#C65B3E', '#2E3B4E', 'var(--st-completed)', 166],
  ['#8E2B22', '#E8DCC4', 'var(--st-in-progress)', 200],
  ['#B8232A', '#141414', 'var(--st-planned)', 166],
  ['#3A2D6B', '#E86A8E', 'var(--st-completed)', 186],
  ['#2E6B57', '#E2403C', 'var(--st-in-progress)', 200],
  ['#15191E', '#B83A2E', 'var(--st-completed)', 186],
  ['#DDE6F2', '#2F6BD8', 'var(--st-planned)', 166],
  ['#F2E64A', '#1C1C24', 'var(--st-completed)', 200],
  ['#8FBCE6', '#4E8A3A', 'var(--st-on-hold)', 186],
  ['#F4A3C0', '#2B2B6E', 'var(--st-completed)', 200],
  ['#E9ECEF', '#1E3F8C', 'var(--st-planned)', 166],
  ['#2B2F4A', '#C9474F', 'var(--st-in-progress)', 186],
  ['#E3D6BF', '#8C2F39', 'var(--st-in-progress)', 200],
  ['#3E4A2F', '#C8A24A', 'var(--st-completed)', 186],
  ['#F06A3B', '#2A1F4E', 'var(--st-wishlist)', 166],
  ['#1F2A30', '#D9D4C7', 'var(--st-planned)', 200],
  ['#3B1E3F', '#7FD6A4', 'var(--st-planned)', 186],
  ['#20305A', '#F08A3E', 'var(--st-completed)', 166],
]

function DecorShelf() {
  return (
    <div aria-hidden="true" className={styles.decor}>
      <div className={styles.decorRow}>
        {SHELF.map(([c1, c2, band, h], i) => (
          <span
            key={i}
            className={styles.decorSpine}
            style={{
              width: h === 166 ? 40 : 46,
              height: h,
              borderTopColor: band,
              background: `linear-gradient(90deg, ${c1} 0 80%, ${c2} 80% 100%)`,
            }}
          />
        ))}
      </div>
      <div className={styles.board} />
    </div>
  )
}

interface FormState {
  username: string
  email: string
  password: string
}

export function LoginPage() {
  const me = useMe()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const location = useLocation()
  const from = (location.state as { from?: string } | null)?.from ?? '/'

  const [mode, setMode] = useState<Mode>('login')
  const [form, setForm] = useState<FormState>({ username: '', email: '', password: '' })
  const refs = {
    username: useRef<HTMLInputElement>(null),
    email: useRef<HTMLInputElement>(null),
    password: useRef<HTMLInputElement>(null),
  }

  useEffect(() => {
    document.title = (mode === 'login' ? 'Logowanie' : 'Nowe konto') + ' · GameLogg'
  }, [mode])

  const submit = useMutation<User, ApiError, Mode>({
    mutationFn: (m) =>
      m === 'login' ? api.login(form.email, form.password) : api.register(form.username, form.email, form.password),
    onSuccess: (user) => {
      queryClient.setQueryData(meKey, user)
      navigate(from, { replace: true })
    },
    onError: (err) => {
      const field = err.field as Field | undefined
      if (field && refs[field]) refs[field].current?.focus()
    },
  })

  // Ktoś już zalogowany nie ma tu czego szukać.
  if (me.data) return <Navigate to={from} replace />

  const errorField = submit.error?.field as Field | undefined
  const generalError = submit.error?.code === 'invalid_credentials'

  function onSubmit(e: FormEvent) {
    e.preventDefault()
    submit.mutate(mode)
  }

  function switchMode(m: Mode) {
    setMode(m)
    submit.reset()
  }

  function input(field: Field, label: string, type: string, autoComplete: string, placeholder?: string) {
    const invalid = errorField === field || (generalError && field !== 'username')
    return (
      <label className={styles.label}>
        {label}
        <input
          ref={refs[field]}
          className={styles.field}
          type={type}
          name={field}
          autoComplete={autoComplete}
          placeholder={placeholder}
          value={form[field]}
          onChange={(e) => setForm((f) => ({ ...f, [field]: e.target.value }))}
          aria-invalid={invalid || undefined}
          aria-describedby={invalid ? 'auth-error' : undefined}
          required
        />
      </label>
    )
  }

  return (
    <div className={styles.screen}>
      <div className={styles.hero}>
        <div>
          <p className={styles.brand}>gamelogg</p>
          <p className={styles.tagline}>Twoje gry na jednej półce, na twoim własnym serwerze.</p>
        </div>
        <DecorShelf />
      </div>

      <main className={styles.panel}>
        <div className={styles.panelInner}>
          <div role="group" aria-label="Formularz" className={styles.tabs}>
            <button type="button" aria-pressed={mode === 'login'} onClick={() => switchMode('login')}>
              Logowanie
            </button>
            <button type="button" aria-pressed={mode === 'register'} onClick={() => switchMode('register')}>
              Nowe konto
            </button>
          </div>

          <form className={styles.form} onSubmit={onSubmit} noValidate>
            <h1 className={styles.title}>{mode === 'login' ? 'Wróć do półki' : 'Załóż konto'}</h1>

            {mode === 'register' && input('username', 'Nazwa użytkownika', 'text', 'username')}
            {input('email', 'Email', 'email', 'email', 'jan@example.com')}
            {input('password', 'Hasło', 'password', mode === 'login' ? 'current-password' : 'new-password')}

            <p id="auth-error" role="alert" className={styles.error}>
              {submit.error?.message ?? ''}
            </p>

            <button type="submit" className={`btn btn-primary ${styles.submit}`} aria-busy={submit.isPending}>
              {mode === 'login' ? 'Zaloguj się' : 'Załóż konto'}
            </button>

            {mode === 'register' && (
              <p className={styles.hint}>
                Konto istnieje tylko na tym serwerze. Grasz sam? Ustaw <strong>AUTH_ENABLED=false</strong> w .env i
                pomiń logowanie.
              </p>
            )}
          </form>
        </div>
      </main>
    </div>
  )
}

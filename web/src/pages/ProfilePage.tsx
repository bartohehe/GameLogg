import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useId, useRef, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { Board } from '../components/Shelf'
import { Spine } from '../components/Spine'
import { api, ApiError } from '../lib/api'
import { monthYearGenitive } from '../lib/format'
import { useStats } from '../lib/games'
import { gameSpineSize } from '../lib/platforms'
import { profileSentence } from '../lib/profile'
import { meKey, useConfig, useMe } from '../lib/session'
import styles from './ProfilePage.module.css'

function initial(name: string) {
  return (name.trim()[0] ?? '?').toLocaleUpperCase('pl-PL')
}

export function ProfilePage() {
  const config = useConfig()
  const me = useMe()
  const stats = useStats()

  useEffect(() => {
    document.title = 'Profil · GameLogg'
  }, [])

  const user = me.data
  if (!user) return null
  const s = stats.data

  return (
    <div className={`page ${styles.page}`}>
      <div className={styles.head}>
        <div aria-hidden="true" className={styles.avatar}>
          {initial(user.username)}
        </div>
        <div>
          <h1 className={styles.name}>{user.username}</h1>
          <p className={styles.since}>
            @{user.username}, gry na półkach od {monthYearGenitive(new Date(user.createdAt))}
          </p>
        </div>
      </div>

      {s && (
        <p className={styles.sentence}>
          {profileSentence({
            total: s.total,
            completed: s.counts.completed ?? 0,
            planned: s.counts.planned ?? 0,
            averageScore: s.averageScore,
          }).map((p, i) =>
            typeof p === 'string' ? (
              p
            ) : (
              <strong key={i} className={p.score ? styles.scoreStrong : styles.strong}>
                {p.strong}
              </strong>
            ),
          )}
        </p>
      )}
      {stats.isError && (
        <p role="alert" className={styles.error}>
          {stats.error.message}
        </p>
      )}

      <div className={styles.columns}>
        <div className={styles.left}>
          <section aria-labelledby="top">
            <h2 id="top" className={styles.h2}>
              Najwyższa półka
            </h2>
            {s && s.top.length > 0 ? (
              <>
                <div className={styles.topShelf}>
                  {s.top.map((e) => (
                    <Spine
                      key={e.id}
                      href={`/game/${e.game.igdbId}`}
                      title={e.game.title}
                      status={e.status}
                      platform={e.platform}
                      score={e.score}
                      colors={e.game.spine}
                      size={{ width: 44, height: gameSpineSize(e.platform).height }}
                    />
                  ))}
                </div>
                <Board />
                <p className={styles.muted}>
                  {s.top.length === 8 ? 'Osiem najwyżej ocenionych gier, od lewej.' : 'Najwyżej ocenione gry, od lewej.'}
                </p>
              </>
            ) : (
              <p className={styles.muted}>Tu staną najwyżej ocenione gry. Oceń kilka ukończonych w rankingach.</p>
            )}
          </section>

          {config.authEnabled && <AccountSection />}
        </div>

        <ServerSection />
      </div>
    </div>
  )
}

function AccountSection() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [changing, setChanging] = useState(false)
  const [done, setDone] = useState(false)
  const changeRef = useRef<HTMLButtonElement>(null)

  const logout = useMutation({
    mutationFn: api.logout,
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: meKey })
      navigate('/login', { replace: true })
    },
  })

  return (
    <section aria-labelledby="account" className={styles.account}>
      <h2 id="account" className={styles.h2}>
        Konto
      </h2>
      {done && (
        <p role="status" className={styles.success}>
          Hasło zmienione. Inne zalogowane urządzenia wylogują się przy następnym odświeżeniu.
        </p>
      )}
      {changing ? (
        <PasswordForm
          onDone={() => {
            setChanging(false)
            setDone(true)
          }}
          onCancel={() => {
            setChanging(false)
            requestAnimationFrame(() => changeRef.current?.focus())
          }}
        />
      ) : (
        <div className={styles.actions}>
          <button
            ref={changeRef}
            type="button"
            className="btn btn-ghost"
            onClick={() => {
              setDone(false)
              setChanging(true)
            }}
          >
            Zmień hasło
          </button>
          <button type="button" className="btn btn-ghost" aria-busy={logout.isPending} onClick={() => logout.mutate()}>
            Wyloguj się
          </button>
        </div>
      )}
      {logout.isError && (
        <p role="alert" className={styles.error}>
          {logout.error.message}
        </p>
      )}
    </section>
  )
}

type PwField = 'currentPassword' | 'newPassword' | 'repeat'

function PasswordForm({ onDone, onCancel }: { onDone: () => void; onCancel: () => void }) {
  const id = useId()
  const [values, setValues] = useState({ currentPassword: '', newPassword: '', repeat: '' })
  const [error, setError] = useState<{ field?: PwField; message: string } | null>(null)
  const [pending, setPending] = useState(false)
  const refs = {
    currentPassword: useRef<HTMLInputElement>(null),
    newPassword: useRef<HTMLInputElement>(null),
    repeat: useRef<HTMLInputElement>(null),
  }

  useEffect(() => {
    refs.currentPassword.current?.focus()
    // Tylko przy otwarciu formularza.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function fail(field: PwField | undefined, message: string) {
    setError({ field, message })
    if (field) refs[field].current?.focus()
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (values.newPassword !== values.repeat) {
      fail('repeat', 'Nowe hasło i jego powtórzenie się różnią.')
      return
    }
    setPending(true)
    try {
      await api.changePassword(values.currentPassword, values.newPassword)
      onDone()
    } catch (err) {
      const field = err instanceof ApiError ? (err.field as PwField | undefined) : undefined
      fail(field, err instanceof Error ? err.message : 'Nie udało się zmienić hasła.')
      setPending(false)
    }
  }

  const field = (name: PwField, label: string, autoComplete: string) => (
    <label className={styles.label}>
      {label}
      <input
        ref={refs[name]}
        className={styles.field}
        type="password"
        autoComplete={autoComplete}
        value={values[name]}
        onChange={(e) => setValues((v) => ({ ...v, [name]: e.target.value }))}
        aria-invalid={error?.field === name || undefined}
        aria-describedby={error?.field === name ? `${id}-err` : undefined}
        required
      />
    </label>
  )

  return (
    <form className={styles.pwForm} onSubmit={submit} noValidate>
      {field('currentPassword', 'Obecne hasło', 'current-password')}
      {field('newPassword', 'Nowe hasło (co najmniej 8 znaków)', 'new-password')}
      {field('repeat', 'Powtórz nowe hasło', 'new-password')}
      <p id={`${id}-err`} role="alert" className={styles.error}>
        {error?.message ?? ''}
      </p>
      <div className={styles.actions}>
        <button type="submit" className="btn btn-primary" aria-busy={pending}>
          Zapisz nowe hasło
        </button>
        <button type="button" className="btn btn-ghost" onClick={onCancel}>
          Anuluj
        </button>
      </div>
    </form>
  )
}

function ServerSection() {
  const config = useConfig()
  const flags: Array<{ label: string; env: string; on: boolean; onText: string; offText: string }> = [
    { label: 'Logowanie i konta', env: 'AUTH_ENABLED', on: config.authEnabled, onText: 'Włączone', offText: 'Wyłączone' },
    { label: 'Filmy, seriale i anime', env: 'MEDIA_ENABLED', on: config.mediaEnabled, onText: 'Włączone', offText: 'Wyłączone' },
    { label: 'Tłumaczenie opisów', env: 'TRANSLATION_ENABLED', on: config.translationEnabled, onText: 'Włączone', offText: 'Wyłączone' },
    { label: 'Gry z IGDB', env: 'IGDB_CLIENT_ID, IGDB_CLIENT_SECRET', on: config.igdbConfigured, onText: 'Klucze są', offText: 'Brak kluczy' },
    { label: 'Filmy i seriale z TMDB', env: 'TMDB_API_KEY', on: config.tmdbConfigured, onText: 'Klucz jest', offText: 'Brak klucza' },
  ]
  return (
    <section aria-labelledby="srv" className={styles.server}>
      <h2 id="srv" className={styles.h2}>
        Ten serwer
      </h2>
      <p className={styles.muted}>
        Funkcje włącza się w pliku <code>.env</code>. Po zmianie uruchom ponownie kontener <code>app</code>.
      </p>
      <ul className={styles.flags}>
        {flags.map((f) => (
          <li key={f.env}>
            <div className={styles.flagText}>
              <div className={styles.flagLabel}>{f.label}</div>
              <code>{f.env}</code>
            </div>
            <span className={f.on ? styles.pillOn : styles.pillOff}>{f.on ? f.onText : f.offText}</span>
          </li>
        ))}
      </ul>
      <p className={styles.version}>GameLogg {config.version}</p>
    </section>
  )
}

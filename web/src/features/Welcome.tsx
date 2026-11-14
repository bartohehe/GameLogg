import { useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { SearchIcon } from '../components/icons'
import { Board } from '../components/Shelf'
import { configKey, useConfig } from '../lib/session'
import styles from './Welcome.module.css'

const GHOSTS = [168, 180, 160, 186, 172, 190, 164, 178, 184, 158, 176, 188, 170, 182, 166, 180, 174, 162]

/** Ekran pierwszego uruchomienia: pusta biblioteka. */
export function Welcome() {
  const config = useConfig()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const [q, setQ] = useState('')
  const [checking, setChecking] = useState(false)
  const ok = config.igdbConfigured

  async function recheck() {
    setChecking(true)
    await qc.refetchQueries({ queryKey: configKey })
    setChecking(false)
  }

  function submit(e: FormEvent) {
    e.preventDefault()
    if (q.trim()) navigate(`/search?q=${encodeURIComponent(q.trim())}`, { state: { seed: Date.now() } })
  }

  return (
    <div className={`page ${styles.page}`}>
      <div aria-hidden="true" className={styles.ghosts}>
        {GHOSTS.map((h, i) => (
          <span key={i} className={styles.ghost} style={{ height: h }} />
        ))}
      </div>
      <Board />

      <div className={styles.grid}>
        <div className={styles.intro}>
          <h1 className={styles.title}>Pusta półka</h1>
          <p className={styles.lead}>
            GameLogg działa na twoim serwerze. Zostały dwa kroki, żeby postawić na półce pierwszą grę.
          </p>
        </div>

        <ol className={styles.steps}>
          <li className={`${styles.step} ${ok ? '' : styles.stepWarn}`}>
            <span className={`${styles.badge} ${ok ? styles.badgeOk : styles.badgeWarn}`}>
              {ok ? (
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="2.2" aria-hidden="true">
                  <path d="M2.5 7.5l3 3 6-7" />
                </svg>
              ) : (
                '1'
              )}
              <span className="sr-only">{ok ? 'Krok 1 gotowy' : 'Krok 1'}</span>
            </span>
            <div className={styles.stepBody}>
              <h2 className={styles.stepTitle}>Połącz z IGDB</h2>
              {ok ? (
                <p className={styles.muted}>
                  Klucze IGDB są w pliku <code>.env</code>.
                </p>
              ) : (
                <>
                  <p className={styles.warn}>
                    Wyszukiwanie gier nie zadziała: brak <code>IGDB_CLIENT_ID</code> i <code>IGDB_CLIENT_SECRET</code>.
                  </p>
                  <p className={styles.text}>
                    Załóż aplikację w Twitch Developer Console, wpisz oba klucze do <code>.env</code> i uruchom ponownie
                    kontener <code>app</code>.
                  </p>
                  <div className={styles.row}>
                    <a
                      className="btn btn-ghost"
                      href="https://github.com/bartohehe/GameLogg#klucze-igdb"
                      target="_blank"
                      rel="noreferrer"
                    >
                      Instrukcja w README
                    </a>
                    <button type="button" className={`btn ${styles.light}`} aria-busy={checking} onClick={recheck}>
                      Sprawdź ponownie
                    </button>
                  </div>
                </>
              )}
            </div>
          </li>

          <li className={`${styles.step} ${ok ? styles.stepActive : styles.stepDisabled}`}>
            <span className={`${styles.badge} ${ok ? styles.badgeNext : styles.badgeIdle}`}>
              2<span className="sr-only">. krok</span>
            </span>
            <div className={styles.stepBody}>
              <h2 className={styles.stepTitle}>Dodaj grę, w którą grasz teraz</h2>
              <p className={styles.muted}>Albo ostatnią, którą skończyłeś. Od niej zaczniesz rankingi.</p>
              <form role="search" onSubmit={submit} className={styles.search}>
                <label className={styles.field}>
                  <SearchIcon />
                  <span className="sr-only">Szukaj gry</span>
                  <input
                    type="search"
                    placeholder="Wpisz tytuł gry"
                    value={q}
                    onChange={(e) => setQ(e.target.value)}
                    disabled={!ok}
                  />
                </label>
              </form>
            </div>
          </li>
        </ol>
      </div>
    </div>
  )
}

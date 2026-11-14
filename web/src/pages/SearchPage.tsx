import { useQuery } from '@tanstack/react-query'
import { useCallback, useEffect, useState } from 'react'
import { Link, useLocation, useSearchParams } from 'react-router'
import { Cover } from '../components/Cover'
import { SearchIcon } from '../components/icons'
import { AddGameModal } from '../features/AddGameModal'
import { ApiError } from '../lib/api'
import { gamesApi, type SearchResult } from '../lib/games'
import { countResults } from '../lib/plural'
import { useConfig } from '../lib/session'
import { STATUS } from '../lib/status'
import styles from './SearchPage.module.css'

const MIN_QUERY = 2
const DEBOUNCE_MS = 300

type Tab = 'games' | 'media'

/** Pole wyszukiwania z debounce: po 300 ms bez pisania zapisuje q w adresie. */
function SearchField({ initial, count, onQuery }: { initial: string; count: number | null; onQuery: (q: string) => void }) {
  const [text, setText] = useState(initial)

  useEffect(() => {
    const t = setTimeout(() => onQuery(text.trim()), DEBOUNCE_MS)
    return () => clearTimeout(t)
  }, [text, onQuery])

  return (
    <div className={styles.field}>
      <SearchIcon size={22} />
      <input
        id="q"
        type="search"
        value={text}
        onChange={(e) => setText(e.target.value)}
        autoComplete="off"
        autoFocus={initial === ''}
        aria-describedby="search-count"
      />
      <span id="search-count" className={`${styles.count} num`} aria-live="polite">
        {count != null ? countResults(count) : ''}
      </span>
    </div>
  )
}

function meta(r: SearchResult) {
  return [r.year, r.developer].filter(Boolean).join(', ')
}

function ResultRow({ r, onAdd }: { r: SearchResult; onAdd: (r: SearchResult) => void }) {
  return (
    <li className={styles.row}>
      <Cover className={styles.thumb} title={r.title} src={r.coverUrl} showTitle={false} />
      <div className={styles.info}>
        <Link to={`/game/${r.igdbId}`} className={styles.rowTitle}>
          {r.title}
        </Link>
        {meta(r) && <div className={styles.meta}>{meta(r)}</div>}
        {r.platforms.length > 0 && <div className={styles.extra}>{r.platforms.join(', ')}</div>}
      </div>
      <div className={styles.score}>
        <div className={`${styles.scoreValue} num`}>{r.igdbRating ?? '–'}</div>
        <div className={styles.scoreSrc}>IGDB</div>
      </div>
      <div className={styles.action}>
        {r.entry ? (
          <Link to={`/library?game=${r.entry.id}`} className={styles.owned}>
            <span aria-hidden="true" className={styles.dot} style={{ background: STATUS[r.entry.status].color }} />
            Na półce: {STATUS[r.entry.status].label}
          </Link>
        ) : (
          <button type="button" className="btn btn-primary" onClick={() => onAdd(r)}>
            Dodaj na półkę
            <span className="sr-only">: {r.title}</span>
          </button>
        )}
      </div>
    </li>
  )
}

function GameResults({ q, onCount }: { q: string; onCount: (n: number | null) => void }) {
  const [adding, setAdding] = useState<SearchResult | null>(null)
  const enabled = q.length >= MIN_QUERY
  const search = useQuery({
    queryKey: ['search', 'games', q],
    queryFn: () => gamesApi.search(q),
    enabled,
    staleTime: 60_000,
    retry: (n, err) => !(err instanceof ApiError && err.status < 500) && n < 1,
  })

  const count = enabled && search.data ? search.data.length : null
  useEffect(() => onCount(count), [count, onCount])

  if (!enabled) {
    return <p className={styles.hint}>Wpisz co najmniej dwa znaki tytułu.</p>
  }
  if (search.isPending) {
    return (
      <p className={styles.hint} role="status">
        Szukam w IGDB…
      </p>
    )
  }
  if (search.isError) {
    return (
      <p className={styles.error} role="alert">
        {search.error.message}
      </p>
    )
  }
  if (search.data.length === 0) {
    return (
      <p className={styles.hint} role="status">
        Nic nie znaleziono dla „{q}”. Sprawdź pisownię albo wpisz krótszy fragment tytułu.
      </p>
    )
  }
  return (
    <>
      <ol className={styles.results}>
        {search.data.map((r) => (
          <ResultRow key={r.igdbId} r={r} onAdd={setAdding} />
        ))}
      </ol>
      <p className={styles.foot}>Nie ma tego, czego szukasz? IGDB zna głównie wydane gry i zapowiedzi z datą.</p>
      {adding && <AddGameModal game={adding} onClose={() => setAdding(null)} />}
    </>
  )
}

export function SearchPage() {
  const config = useConfig()
  const location = useLocation()
  const [params, setParams] = useSearchParams()
  const q = (params.get('q') ?? '').trim()
  const tab: Tab = config.mediaEnabled && params.get('tab') === 'media' ? 'media' : 'games'
  const [count, setCount] = useState<number | null>(null)

  useEffect(() => {
    document.title = (q ? `${q} · ` : '') + 'Szukaj · GameLogg'
  }, [q])

  const setQuery = useCallback(
    (next: string) => {
      if (next === q) return
      setParams(
        (p) => {
          if (next) p.set('q', next)
          else p.delete('q')
          return p
        },
        { replace: true },
      )
    },
    [q, setParams],
  )

  const setTab = (t: Tab) =>
    setParams(
      (p) => {
        if (t === 'media') p.set('tab', 'media')
        else p.delete('tab')
        return p
      },
      { replace: true },
    )

  // Nowe wyszukiwanie z nagłówka (state.seed) odtwarza pole z q z adresu.
  const seed = (location.state as { seed?: number } | null)?.seed ?? 0

  return (
    <div className={`page ${styles.page}`}>
      <label htmlFor="q" className={styles.label}>
        Czego szukasz?
      </label>
      <SearchField key={seed} initial={q} count={tab === 'games' ? count : null} onQuery={setQuery} />

      <div role="group" aria-label="Szukaj w" className={styles.tabs}>
        <button type="button" aria-pressed={tab === 'games'} onClick={() => setTab('games')}>
          Gry, z IGDB
        </button>
        {config.mediaEnabled && (
          <button type="button" aria-pressed={tab === 'media'} onClick={() => setTab('media')}>
            Filmy, seriale i anime
          </button>
        )}
      </div>

      {tab === 'games' ? (
        config.igdbConfigured ? (
          <GameResults q={q} onCount={setCount} />
        ) : (
          <p className={styles.error} role="alert">
            Wyszukiwanie gier nie zadziała: brak IGDB_CLIENT_ID i IGDB_CLIENT_SECRET w .env. Dopisz oba klucze i
            uruchom ponownie kontener app.
          </p>
        )
      ) : (
        <p className={styles.hint}>Wyszukiwanie filmów, seriali i anime pojawi się w etapie 5.</p>
      )}
    </div>
  )
}

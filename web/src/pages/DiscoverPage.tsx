import { useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { Cover } from '../components/Cover'
import { PlusIcon } from '../components/icons'
import { Board } from '../components/Shelf'
import { Spine } from '../components/Spine'
import { AddGameModal } from '../features/AddGameModal'
import { NoteModal } from '../features/NoteModal'
import { targetFromEntry, useOpenRanking } from '../features/ranking/context'
import { Welcome } from '../features/Welcome'
import { ApiError } from '../lib/api'
import { dayMonth } from '../lib/format'
import {
  discoverApi,
  useAddGame,
  useLibrary,
  useStats,
  useUpdateEntry,
  type Entry,
  type Recommendation,
  type SearchResult,
  type Stats,
} from '../lib/games'
import { countGamesAcc, plural } from '../lib/plural'
import { useConfig } from '../lib/session'
import { SHELF_ORDER, STATUS, type Status } from '../lib/status'
import styles from './DiscoverPage.module.css'

const noRetryOn4xx = (n: number, err: Error) => !(err instanceof ApiError && err.status < 500) && n < 1

export function DiscoverPage() {
  const lib = useLibrary()

  useEffect(() => {
    document.title = 'Odkryj · GameLogg'
  }, [])

  if (lib.isPending) return <div className="page" aria-busy="true" />
  if (lib.isSuccess && lib.data.length === 0) return <Welcome />
  const entries = lib.data ?? []

  return (
    <div className={`page ${styles.page}`}>
      <div className={styles.top}>
        <NowPlaying entries={entries} />
        <YearPanel />
      </div>
      <Popular />
      <Recommendations entries={entries} />
    </div>
  )
}

// --- Grasz teraz ---

function NowPlaying({ entries }: { entries: Entry[] }) {
  const playing = entries
    .filter((e) => e.status === 'in_progress')
    .sort((a, b) => a.addedAt.localeCompare(b.addedAt))
  const update = useUpdateEntry()
  const openRanking = useOpenRanking()
  const [noteFor, setNoteFor] = useState<Entry | null>(null)

  function complete(e: Entry) {
    update.mutate(
      { id: e.id, patch: { status: 'completed' } },
      { onSuccess: (done) => openRanking(targetFromEntry(done)) },
    )
  }

  return (
    <section aria-labelledby="now" className={styles.now}>
      <h1 id="now" className={styles.hero}>
        {playing.length > 0 ? `Grasz teraz w ${countGamesAcc(playing.length)}` : 'Nic teraz nie grasz'}
      </h1>
      <p className={styles.heroLead}>
        {playing.length > 0
          ? 'Skończysz którąś? Oznacz ją jako ukończoną, a od razu ustawisz ją w swoich rankingach.'
          : 'Postaw grę na półce W trakcie, a pojawi się tutaj razem z szybką notatką.'}
      </p>
      {playing.length === 0 ? (
        <Link to="/library" className="btn btn-ghost">
          Wybierz z biblioteki
        </Link>
      ) : (
        <ul className={styles.nowGrid}>
          {playing.map((e) => (
            <li key={e.id}>
              <article>
                <Link to={`/game/${e.game.igdbId}`} className={styles.coverLink} tabIndex={-1} aria-hidden="true">
                  <Cover className={styles.nowCover} title={e.game.title} src={e.game.coverUrl} colors={e.game.spine} />
                </Link>
                <h2 className={styles.nowTitle}>
                  <Link to={`/game/${e.game.igdbId}`}>{e.game.title}</Link>
                </h2>
                <p className={styles.muted}>
                  {e.platform ? `${e.platform}, ` : ''}na półce od {dayMonth(new Date(e.addedAt))}
                </p>
                <div className={styles.nowActions}>
                  <button
                    type="button"
                    className={`btn btn-ghost ${styles.small}`}
                    aria-busy={update.isPending && update.variables?.id === e.id}
                    onClick={() => complete(e)}
                  >
                    Ukończona<span className="sr-only">: {e.game.title}</span>
                  </button>
                  <button type="button" className={`btn btn-ghost ${styles.small}`} onClick={() => setNoteFor(e)}>
                    Notatka<span className="sr-only">: {e.game.title}</span>
                  </button>
                </div>
              </article>
            </li>
          ))}
        </ul>
      )}
      {update.isError && (
        <p role="alert" className={styles.error}>
          {update.error.message}
        </p>
      )}
      {noteFor && <NoteModal entry={noteFor} onClose={() => setNoteFor(null)} />}
    </section>
  )
}

// --- Twój rok ---

const STATUS_PHRASE: Record<Status, (n: number) => string> = {
  in_progress: (n) => `${n} w trakcie`,
  planned: (n) => `${n} ${plural(n, 'planowana', 'planowane', 'planowanych')}`,
  completed: (n) => `${n} ${plural(n, 'ukończona', 'ukończone', 'ukończonych')}`,
  on_hold: (n) => `${n} ${plural(n, 'wstrzymana', 'wstrzymane', 'wstrzymanych')}`,
  wishlist: (n) => `${n} na liście życzeń`,
  abandoned: (n) => `${n} ${plural(n, 'porzucona', 'porzucone', 'porzuconych')}`,
}

function yearSentence(s: Stats) {
  const y = s.year
  if (y.completed === 0 && y.abandoned === 0) return 'W tym roku nic jeszcze nie trafiło na półkę Ukończone.'
  const parts = [`${y.completed} ${plural(y.completed, 'gra ukończona', 'gry ukończone', 'gier ukończonych')}`]
  if (y.abandoned > 0) parts.push(`${y.abandoned} ${plural(y.abandoned, 'porzucona', 'porzucone', 'porzuconych')}`)
  return parts.join(', ') + '.' + (y.averageScore != null ? ` Średnia ocena ${y.averageScore}.` : '')
}

function YearPanel() {
  const stats = useStats()
  const s = stats.data
  const year = new Date().getFullYear()

  return (
    <aside aria-labelledby="year" className={styles.year}>
      <div>
        <h2 id="year" className={styles.yearTitle}>
          Twój {year}
        </h2>
        <p className={styles.muted}>{s ? yearSentence(s) : stats.isError ? stats.error.message : ' '}</p>
      </div>
      {s && (
        <>
          <div>
            <div
              role="img"
              aria-label={`Biblioteka według statusu: ${SHELF_ORDER.map((st) => STATUS_PHRASE[st](s.counts[st] ?? 0)).join(', ')}`}
              className={styles.dist}
            >
              {SHELF_ORDER.filter((st) => (s.counts[st] ?? 0) > 0).map((st) => (
                <span key={st} style={{ flex: s.counts[st], background: STATUS[st].color }} />
              ))}
            </div>
            <ul className={styles.legend}>
              {SHELF_ORDER.map((st) => (
                <li key={st}>
                  <span aria-hidden="true" className={styles.legendDot} style={{ background: STATUS[st].color }} />
                  <span className={styles.legendLabel}>{STATUS[st].label}</span>
                  <span className={`${styles.muted} num`}>{s.counts[st] ?? 0}</span>
                </li>
              ))}
            </ul>
          </div>
          <div className={styles.yearTop}>
            <h3 className={styles.h3}>Najwyżej w tym roku</h3>
            {s.year.top.length === 0 ? (
              <p className={styles.muted}>Tu staną najwyżej ocenione gry ukończone w tym roku.</p>
            ) : (
              <>
                <div className={styles.miniShelf}>
                  {s.year.top.map((e) => (
                    <Spine
                      key={e.id}
                      href={`/game/${e.game.igdbId}`}
                      title={e.game.title}
                      status={e.status}
                      platform=""
                      label=""
                      score={e.score}
                      colors={e.game.spine}
                      size={{ width: 42, height: Math.max(120, Math.min(190, 150 + ((e.score ?? 90) - 90) * 4)) }}
                    />
                  ))}
                </div>
                <Board thin />
              </>
            )}
          </div>
        </>
      )}
    </aside>
  )
}

// --- Popularne teraz ---

function Popular() {
  const config = useConfig()
  const [adding, setAdding] = useState<SearchResult | null>(null)
  const popular = useQuery({
    queryKey: ['popular'],
    queryFn: discoverApi.popular,
    enabled: config.igdbConfigured,
    staleTime: 10 * 60_000,
    retry: noRetryOn4xx,
  })

  return (
    <section aria-labelledby="pop">
      <div className={styles.sectionHead}>
        <h2 id="pop" className={styles.h2}>
          Popularne teraz
        </h2>
        <p className={styles.muted}>Premiery z ostatnich 30 dni, według ocen w IGDB.</p>
      </div>
      {!config.igdbConfigured ? (
        <p className={styles.error}>
          Popularne premiery nie zadziałają: brak IGDB_CLIENT_ID i IGDB_CLIENT_SECRET w .env.
        </p>
      ) : popular.isPending ? (
        <p role="status" className={styles.muted}>
          Wczytuję premiery…
        </p>
      ) : popular.isError ? (
        <p role="alert" className={styles.error}>
          {popular.error.message}
        </p>
      ) : popular.data.length === 0 ? (
        <p className={styles.muted}>IGDB nie ma ocenionych premier z ostatnich 30 dni.</p>
      ) : (
        <ul className={styles.popGrid}>
          {popular.data.map((g) => (
            <li key={g.igdbId}>
              <article>
                <Link to={`/game/${g.igdbId}`} tabIndex={-1} aria-hidden="true" className={styles.coverLink}>
                  <Cover className={styles.popCover} title={g.title} src={g.coverUrl} colors={g.spine} />
                </Link>
                <h3 className={styles.popTitle}>
                  <Link to={`/game/${g.igdbId}`}>{g.title}</Link>
                </h3>
                <div className={styles.popMeta}>
                  <span className={`${styles.muted} num`}>{g.igdbRating != null ? `IGDB ${g.igdbRating}` : 'IGDB –'}</span>
                  {g.entry ? (
                    <Link
                      to={`/library?game=${g.entry.id}`}
                      className={styles.ownedDot}
                      aria-label={`Na półce: ${STATUS[g.entry.status].label}`}
                      title={`Na półce: ${STATUS[g.entry.status].label}`}
                    >
                      <span style={{ background: STATUS[g.entry.status].color }} />
                    </Link>
                  ) : (
                    <button
                      type="button"
                      className={styles.plus}
                      aria-label={`Dodaj na półkę: ${g.title}`}
                      onClick={() => setAdding(g)}
                    >
                      <PlusIcon size={14} />
                    </button>
                  )}
                </div>
              </article>
            </li>
          ))}
        </ul>
      )}
      {adding && <AddGameModal game={adding} onClose={() => setAdding(null)} />}
    </section>
  )
}

// --- Może ci się spodobać ---

/** Platforma, na której gracz gra najczęściej, o ile gra na nią wyszła. */
function preferredPlatform(platforms: string[], entries: Entry[]) {
  const counts = new Map<string, number>()
  for (const e of entries) if (e.platform) counts.set(e.platform, (counts.get(e.platform) ?? 0) + 1)
  const ranked = [...platforms].sort((a, b) => (counts.get(b) ?? 0) - (counts.get(a) ?? 0))
  return ranked[0] ?? ''
}

function Recommendations({ entries }: { entries: Entry[] }) {
  const config = useConfig()
  const recs = useQuery({
    queryKey: ['recommendations'],
    queryFn: discoverApi.recommendations,
    enabled: config.igdbConfigured,
    staleTime: 10 * 60_000,
    retry: noRetryOn4xx,
  })

  let body: React.ReactNode
  if (!config.igdbConfigured) {
    body = <p className={styles.error}>Polecenia nie zadziałają bez kluczy IGDB w .env.</p>
  } else if (recs.isPending) {
    body = (
      <p role="status" className={styles.muted}>
        Szukam podobnych gier…
      </p>
    )
  } else if (recs.isError) {
    body = (
      <p role="alert" className={styles.error}>
        {recs.error.message}
      </p>
    )
  } else if (recs.data.basedOn === 0) {
    body = <p className={styles.muted}>Oceń kilka ukończonych gier w rankingach, a podpowiem podobne.</p>
  } else if (recs.data.items.length === 0) {
    body = <p className={styles.muted}>IGDB nie zna podobnych gier, których jeszcze nie masz.</p>
  } else {
    body = (
      <ul className={styles.recGrid}>
        {recs.data.items.map((r) => (
          <RecommendationCard key={r.game.igdbId} rec={r} platform={preferredPlatform(r.game.platforms, entries)} />
        ))}
      </ul>
    )
  }

  return (
    <section aria-labelledby="rec">
      <div className={styles.sectionHead}>
        <h2 id="rec" className={styles.h2}>
          Może ci się spodobać
        </h2>
        <p className={styles.muted}>Dobrane do gier, które oceniłeś najwyżej.</p>
      </div>
      {body}
    </section>
  )
}

function RecommendationCard({ rec, platform }: { rec: Recommendation; platform: string }) {
  const add = useAddGame()
  const g = rec.game
  return (
    <li className={styles.rec}>
      <Cover className={styles.recThumb} title={g.title} src={g.coverUrl} showTitle={false} />
      <div className={styles.recBody}>
        <h3 className={styles.recTitle}>
          <Link to={`/game/${g.igdbId}`}>{g.title}</Link>
        </h3>
        <p className={styles.muted}>
          Bo dałeś {rec.because.title} ocenę {rec.because.score}.
        </p>
        {add.isSuccess ? (
          <Link to={`/library?game=${add.data.id}`} className={styles.recAdded}>
            Na półce: Planowane
          </Link>
        ) : (
          <button
            type="button"
            className={styles.recAdd}
            aria-busy={add.isPending}
            onClick={() => add.mutate({ igdbId: g.igdbId, status: 'planned', platform })}
          >
            Dodaj do planowanych<span className="sr-only">: {g.title}</span>
          </button>
        )}
        {add.isError && (
          <p role="alert" className={styles.error}>
            {add.error.message}
          </p>
        )}
      </div>
    </li>
  )
}

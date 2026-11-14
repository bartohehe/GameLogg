import { useQuery } from '@tanstack/react-query'
import { useEffect, useId, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { CategoryBreakdown } from '../components/CategoryBreakdown'
import { Cover } from '../components/Cover'
import { BackIcon } from '../components/icons'
import { AddGameModal } from '../features/AddGameModal'
import { targetFromEntry, useOpenRanking } from '../features/ranking/context'
import { ApiError } from '../lib/api'
import { longDate } from '../lib/format'
import {
  gamesApi,
  useEntryRankings,
  useLibrary,
  useRemoveEntry,
  useUpdateEntry,
  type Entry,
  type GameDetails,
} from '../lib/games'
import { STATUS, type Status } from '../lib/status'
import styles from './GamePage.module.css'

/** Kolejność przycisków statusu na karcie gry (jak w mockupie). */
const CARD_ORDER: Status[] = ['wishlist', 'planned', 'in_progress', 'on_hold', 'completed', 'abandoned']

const NOTE_DEBOUNCE_MS = 800

function parseDate(iso: string) {
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(y!, (m ?? 1) - 1, d ?? 1)
}

function completedText(n: number) {
  if (n <= 0) return ''
  if (n === 1) return ' Ukończona raz.'
  if (n === 2) return ' Ukończona dwa razy.'
  return ` Ukończona ${n} razy.`
}

export function GamePage() {
  const { igdbId } = useParams()
  const id = Number(igdbId)
  const game = useQuery({
    queryKey: ['game', id],
    queryFn: () => gamesApi.details(id),
    enabled: Number.isInteger(id) && id > 0,
    retry: (n, err) => !(err instanceof ApiError && err.status < 500) && n < 1,
  })

  useEffect(() => {
    if (game.data) document.title = `${game.data.title} · GameLogg`
  }, [game.data])

  if (game.isPending) return <div className="page" aria-busy="true" />
  if (game.isError) {
    return (
      <div className="page">
        <BackLink entry={null} />
        <h1 className="page-title">Nie da się wczytać gry</h1>
        <p role="alert" className="page-lead" style={{ color: 'var(--warn)' }}>
          {game.error.message}
        </p>
      </div>
    )
  }
  return <GameCard key={game.data.entry?.id ?? 'none'} game={game.data} />
}

function BackLink({ entry }: { entry: Entry | null }) {
  return (
    <Link to={entry ? `/library?game=${entry.id}` : '/library'} className={styles.back}>
      <BackIcon />
      Wróć na półkę
    </Link>
  )
}

function GameCard({ game }: { game: GameDetails }) {
  const entry = game.entry
  const [adding, setAdding] = useState(false)
  const meta = [
    ['Deweloper', game.developer],
    ['Premiera', game.releaseDate ? longDate(parseDate(game.releaseDate)) : ''],
    ['Gatunki', game.genres.join(', ')],
    ['Platformy', game.platforms.join(', ')],
    ['Ocena w IGDB', game.igdbRating != null ? `${game.igdbRating} na 100` : ''],
  ].filter(([, v]) => v)

  return (
    <div className={`page ${styles.page}`}>
      <BackLink entry={entry} />

      <div className={styles.layout}>
        <div className={styles.side}>
          <Cover className={styles.cover} title={game.title} src={game.coverUrl} colors={game.spine} />
          <dl className={styles.meta}>
            {meta.map(([k, v]) => (
              <div key={k}>
                <dt>{k}</dt>
                <dd>{v}</dd>
              </div>
            ))}
          </dl>
        </div>

        <div className={styles.main}>
          <h1 className={styles.title}>{game.title}</h1>
          <p className={styles.sub}>
            {entry
              ? `Na półce od ${longDate(new Date(entry.addedAt))}.${completedText(entry.completedCount)}`
              : 'Tej gry nie ma jeszcze na twojej półce.'}
          </p>

          {entry ? (
            <>
              <StatusSection entry={entry} platforms={game.platforms} />
              <RatingSection entry={entry} />
              <NoteSection entry={entry} />
            </>
          ) : (
            <div className={styles.addRow}>
              <button type="button" className="btn btn-primary" onClick={() => setAdding(true)}>
                Dodaj na półkę
              </button>
            </div>
          )}

          <section aria-labelledby="about" className={styles.about}>
            <h2 id="about" className={styles.h2}>
              O grze
            </h2>
            <p className={styles.summary}>{game.summary || 'IGDB nie ma opisu tej gry.'}</p>
          </section>

          {entry && <RemoveSection entry={entry} />}
        </div>
      </div>

      {adding && <AddGameModal game={game} onClose={() => setAdding(false)} />}
    </div>
  )
}

function StatusSection({ entry, platforms }: { entry: Entry; platforms: string[] }) {
  const update = useUpdateEntry()
  const openRanking = useOpenRanking()
  const [pending, setPending] = useState<Status | null>(null)
  const platformId = useId()

  function change(next: Status) {
    if (next === entry.status) return
    // Wyjście z Ukończonych kasuje ocenę i miejsca w rankingach: najpierw pytamy.
    if (entry.status === 'completed' && entry.score != null && pending !== next) {
      setPending(next)
      return
    }
    setPending(null)
    update.mutate(
      { id: entry.id, patch: { status: next } },
      { onSuccess: (e) => e.status === 'completed' && openRanking(targetFromEntry(e)) },
    )
  }

  return (
    <section aria-labelledby="status-h" className={styles.section}>
      <h2 id="status-h" className={styles.h2}>
        Status
      </h2>
      <div role="group" aria-labelledby="status-h" className={styles.statuses}>
        {CARD_ORDER.map((s) => (
          <button
            key={s}
            type="button"
            className={styles.status}
            aria-pressed={entry.status === s}
            disabled={update.isPending}
            onClick={() => change(s)}
          >
            <span aria-hidden="true" className={styles.dot} style={{ background: STATUS[s].color }} />
            {STATUS[s].label}
          </button>
        ))}
      </div>

      {pending && (
        <div role="alert" className={styles.confirm}>
          <p>
            Zmiana na {STATUS[pending].label} zdejmie grę ze wszystkich rankingów i skasuje ocenę {entry.score}. Oceny
            innych gier przeliczą się od nowa.
          </p>
          <div className={styles.confirmActions}>
            <button type="button" className="btn btn-primary" onClick={() => change(pending)}>
              Zmień na {STATUS[pending].label}
            </button>
            <button type="button" className="btn btn-ghost" onClick={() => setPending(null)}>
              Zostaw Ukończone
            </button>
          </div>
        </div>
      )}

      {platforms.length > 0 && (
        <div className={styles.platformRow}>
          <label htmlFor={platformId}>Grasz na</label>
          <select
            id={platformId}
            value={entry.platform}
            disabled={update.isPending}
            onChange={(e) => update.mutate({ id: entry.id, patch: { platform: e.target.value } })}
          >
            {!platforms.includes(entry.platform) && <option value={entry.platform}>{entry.platform || 'Nie wybrano'}</option>}
            {platforms.map((p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ))}
          </select>
        </div>
      )}
      {update.isError && (
        <p role="alert" className={styles.error}>
          {update.error.message}
        </p>
      )}
    </section>
  )
}

function RatingSection({ entry }: { entry: Entry }) {
  const openRanking = useOpenRanking()
  const completed = entry.status === 'completed'
  const places = useEntryRankings(entry.id, completed)
  const library = useLibrary('score')

  if (!completed) {
    return (
      <section aria-labelledby="rate" className={styles.rating}>
        <h2 id="rate" className={styles.ratingTitle}>
          Twoja ocena
        </h2>
        <p className={styles.muted}>
          Ocena pojawi się, gdy oznaczysz grę jako ukończoną i ustawisz ją w sześciu rankingach kategorii.
        </p>
      </section>
    )
  }

  const scored = library.data?.filter((e) => e.score != null) ?? []
  const rank = entry.score != null ? scored.filter((e) => e.score! > entry.score!).length + 1 : null

  return (
    <section aria-labelledby="rate" className={styles.rating}>
      <div className={styles.ratingHead}>
        <div className={styles.ratingIntro}>
          <h2 id="rate" className={styles.ratingTitle}>
            Twoja ocena
          </h2>
          <p className={styles.muted}>
            Każda kategoria to osobny ranking twoich ukończonych gier. Pierwsze miejsce daje 100 punktów, ostatnie 60.
            Ocena końcowa to średnia ważona.
          </p>
        </div>
        <div className={styles.bigScoreBox}>
          <div className={`${styles.bigScore} num`}>{entry.score ?? '–'}</div>
          {rank != null && <div className={styles.muted}>{rank}. miejsce w bibliotece</div>}
        </div>
      </div>
      {places.data && <CategoryBreakdown categories={places.data.categories} variant="full" />}
      <button type="button" className={`btn btn-primary ${styles.rateBtn}`} onClick={() => openRanking(targetFromEntry(entry))}>
        {entry.score != null ? 'Zmień miejsca w rankingach' : 'Oceń w rankingach'}
      </button>
    </section>
  )
}

function NoteSection({ entry }: { entry: Entry }) {
  const update = useUpdateEntry()
  const [text, setText] = useState(entry.note)
  const [state, setState] = useState<'idle' | 'dirty' | 'saving' | 'saved' | 'error'>('idle')
  const saved = useRef(entry.note)
  const textRef = useRef(text)
  const mutate = useRef(update.mutate)

  useEffect(() => {
    textRef.current = text
    mutate.current = update.mutate
  })

  // Autozapis: 800 ms po ostatnim znaku.
  useEffect(() => {
    if (text === saved.current) return
    const t = setTimeout(() => {
      setState('saving')
      const value = textRef.current
      mutate.current(
        { id: entry.id, patch: { note: value } },
        {
          onSuccess: () => {
            saved.current = value
            setState(textRef.current === value ? 'saved' : 'dirty')
          },
          onError: () => setState('error'),
        },
      )
    }, NOTE_DEBOUNCE_MS)
    return () => clearTimeout(t)
  }, [text, entry.id])

  const status =
    state === 'saving'
      ? 'Zapisuję…'
      : state === 'saved'
        ? 'Zapisano.'
        : state === 'error'
          ? `Nie zapisano: ${update.error?.message ?? 'błąd serwera'}. Zmień tekst, żeby spróbować ponownie.`
          : ''

  return (
    <section className={styles.note}>
      <label htmlFor="notearea" className={styles.h2}>
        Notatka
      </label>
      <textarea
        id="notearea"
        rows={3}
        maxLength={5000}
        value={text}
        placeholder="Co zapamiętałeś z tej gry?"
        aria-describedby="note-status"
        onChange={(e) => {
          setText(e.target.value)
          setState('dirty')
        }}
      />
      <p id="note-status" role="status" className={state === 'error' ? styles.error : styles.noteStatus}>
        {status}
      </p>
    </section>
  )
}

function RemoveSection({ entry }: { entry: Entry }) {
  const remove = useRemoveEntry()
  const navigate = useNavigate()
  const [confirming, setConfirming] = useState(false)

  return (
    <section className={styles.remove}>
      {!confirming ? (
        <button type="button" className={styles.removeBtn} onClick={() => setConfirming(true)}>
          Zdejmij z półki
        </button>
      ) : (
        <div role="alert" className={styles.confirm}>
          <p>
            Gra zniknie z biblioteki razem z notatką
            {entry.score != null ? ' i miejscami w rankingach. Oceny innych gier przeliczą się od nowa.' : '.'}
          </p>
          <div className={styles.confirmActions}>
            <button
              type="button"
              className="btn btn-primary"
              aria-busy={remove.isPending}
              onClick={() => remove.mutate(entry.id, { onSuccess: () => navigate('/library', { replace: true }) })}
            >
              Zdejmij z półki
            </button>
            <button type="button" className="btn btn-ghost" onClick={() => setConfirming(false)}>
              Zostaw
            </button>
          </div>
          {remove.isError && <p className={styles.error}>{remove.error.message}</p>}
        </div>
      )}
    </section>
  )
}

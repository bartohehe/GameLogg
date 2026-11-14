import { useEffect } from 'react'
import { Link, useSearchParams } from 'react-router'
import { Cover } from '../components/Cover'
import { Board, Shelf } from '../components/Shelf'
import { Spine } from '../components/Spine'
import { MediaPanel } from '../features/MediaPanel'
import {
  countTitles,
  MEDIA_STATUS,
  mediaSpineSize,
  TYPE_LABEL,
  useEpisode,
  useMediaList,
  useUpdateMedia,
  type MediaItem,
  type MediaStatus,
  type MediaType,
} from '../lib/media'
import { useMediaQuery } from '../lib/useMediaQuery'
import styles from './MediaPage.module.css'

const TYPES: Array<[MediaType | 'all', string]> = [
  ['all', 'Wszystko'],
  ['film', 'Filmy'],
  ['serial', 'Seriale'],
  ['anime', 'Anime'],
]

function isType(v: string | null): v is MediaType {
  return v === 'film' || v === 'serial' || v === 'anime'
}

function WatchingCard({ m, onSelect }: { m: MediaItem; onSelect: (id: number) => void }) {
  const episode = useEpisode()
  const update = useUpdateMedia()
  const watched = m.watchedEpisodes ?? 0
  const pct = m.episodes ? Math.round((watched / m.episodes) * 100) : 0

  return (
    <article className={styles.card}>
      <Cover className={styles.thumb} title={m.title} src={m.coverUrl} colors={m.spine} showTitle={false} />
      <div className={styles.cardBody}>
        <h3 className={styles.cardTitle}>
          <button type="button" className={styles.titleBtn} onClick={() => onSelect(m.id)}>
            {m.title}
          </button>
        </h3>
        <p className={styles.muted}>{[TYPE_LABEL[m.type], m.creator].filter(Boolean).join(', ')}</p>
        {m.type !== 'film' && (
          <div className={styles.progress}>
            <div className={styles.progressText}>
              <span className="num">{m.episodes ? `Odcinek ${watched} z ${m.episodes}` : `Obejrzane odcinki: ${watched}`}</span>
              {m.episodes != null && <span className={`${styles.muted} num`}>zostało {m.episodes - watched}</span>}
            </div>
            {m.episodes != null && (
              <div aria-hidden="true" className={styles.bar}>
                <span style={{ width: `${pct}%` }} />
              </div>
            )}
          </div>
        )}
        {episode.isError && (
          <p role="alert" className={styles.error}>
            {episode.error.message}
          </p>
        )}
      </div>
      {m.type === 'film' ? (
        <button
          type="button"
          className={`btn btn-ghost ${styles.ep}`}
          aria-busy={update.isPending}
          onClick={() => update.mutate({ id: m.id, patch: { status: 'watched' } })}
        >
          Obejrzane<span className="sr-only">: {m.title}</span>
        </button>
      ) : (
        <button
          type="button"
          className={`btn btn-ghost ${styles.ep}`}
          aria-busy={episode.isPending}
          onClick={() => episode.mutate(m.id)}
        >
          +1 odcinek<span className="sr-only">: {m.title}</span>
        </button>
      )}
    </article>
  )
}

function MediaShelf({
  status,
  items,
  selectedId,
  onSelect,
}: {
  status: MediaStatus
  items: MediaItem[]
  selectedId: number | null
  onSelect: (id: number) => void
}) {
  return (
    <section aria-labelledby={`m-${status}`}>
      <div className={styles.shelfHead}>
        <span aria-hidden="true" className={styles.shelfDot} style={{ background: MEDIA_STATUS[status].color }} />
        <h2 id={`m-${status}`} className={styles.shelfTitle}>
          {MEDIA_STATUS[status].label}
        </h2>
        <span className={`${styles.muted} num`}>{items.length}</span>
      </div>
      {items.length === 0 ? (
        <div>
          <p className={styles.emptyRow}>Na tej półce nic nie stoi.</p>
          <Board />
        </div>
      ) : (
        <Shelf>
          {items.map((m) => (
            <Spine
              key={m.id}
              title={m.title}
              statusLabel={MEDIA_STATUS[m.status].label}
              bandColor={MEDIA_STATUS[m.status].color}
              platform=""
              label={TYPE_LABEL[m.type]}
              score={m.score}
              colors={m.spine}
              size={mediaSpineSize(m.type)}
              selected={m.id === selectedId}
              onSelect={() => onSelect(m.id)}
            />
          ))}
        </Shelf>
      )}
    </section>
  )
}

export function MediaPage() {
  const [params, setParams] = useSearchParams()
  const type = isType(params.get('type')) ? (params.get('type') as MediaType) : null
  const selectedId = Number(params.get('item')) || null
  const isCompact = useMediaQuery('(max-width: 899.98px)')
  const list = useMediaList()

  useEffect(() => {
    document.title = 'Filmy, seriale i anime · GameLogg'
  }, [])

  const update = (key: string, value: string | null) =>
    setParams(
      (p) => {
        if (value == null) p.delete(key)
        else p.set(key, value)
        return p
      },
      { replace: true },
    )
  const select = (id: number) => update('item', String(id))

  if (list.isPending) return <div className="page" aria-busy="true" />
  if (list.isError) {
    return (
      <div className="page">
        <h1 className="page-title">Filmy, seriale i anime</h1>
        <p role="alert" className={styles.error}>
          {list.error.message}
        </p>
      </div>
    )
  }

  const all = list.data
  const items = type ? all.filter((m) => m.type === type) : all
  const watching = items.filter((m) => m.status === 'watching')
  const backlog = items.filter((m) => m.status === 'backlog')
  const watched = items
    .filter((m) => m.status === 'watched')
    .sort((a, b) => (b.score ?? -1) - (a.score ?? -1))
  const selected = all.find((m) => m.id === selectedId) ?? null

  return (
    <div className="page">
      <div className={styles.head}>
        <div>
          <h1 className="page-title">Filmy, seriale i anime</h1>
          <p className="page-lead">
            {all.length > 0
              ? `${countTitles(all.length)}. Przy serialach i anime liczysz odcinki, film wystarczy oznaczyć jako obejrzany.`
              : 'Lista jest pusta. Znajdź film, serial albo anime i dodaj go do listy.'}
          </p>
        </div>
        <div className={styles.headActions}>
          <div role="group" aria-label="Rodzaj" className={styles.seg}>
            {TYPES.map(([t, label]) => (
              <button
                key={t}
                type="button"
                aria-pressed={(type ?? 'all') === t}
                onClick={() => update('type', t === 'all' ? null : t)}
              >
                {label}
              </button>
            ))}
          </div>
          <Link to="/search?tab=media" className="btn btn-primary">
            Dodaj tytuł
          </Link>
        </div>
      </div>

      <div className={styles.layout}>
        <div className={styles.main}>
          {watching.length > 0 && (
            <section aria-labelledby="watching" className={styles.watching}>
              <h2 id="watching" className={styles.h2}>
                Oglądasz
              </h2>
              <ul className={styles.cards}>
                {watching.map((m) => (
                  <li key={m.id}>
                    <WatchingCard m={m} onSelect={select} />
                  </li>
                ))}
              </ul>
            </section>
          )}
          <div className={styles.shelves}>
            <MediaShelf status="backlog" items={backlog} selectedId={selectedId} onSelect={select} />
            <MediaShelf status="watched" items={watched} selectedId={selectedId} onSelect={select} />
          </div>
        </div>

        {selected && !isCompact && <MediaPanel item={selected} variant="aside" onClose={() => update('item', null)} />}
      </div>
      {selected && isCompact && (
        <>
          <div className={styles.sheetSpacer} />
          <MediaPanel item={selected} variant="sheet" onClose={() => update('item', null)} />
        </>
      )}
    </div>
  )
}

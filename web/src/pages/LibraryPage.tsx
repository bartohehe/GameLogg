import { useEffect } from 'react'
import { Link, useSearchParams } from 'react-router'
import { Cover } from '../components/Cover'
import { PlusIcon } from '../components/icons'
import { Board, Shelf } from '../components/Shelf'
import { Spine } from '../components/Spine'
import { GamePanel, GameSheet } from '../features/GamePanel'
import { Welcome } from '../features/Welcome'
import { useLibrary, type Entry, type LibrarySort } from '../lib/games'
import { countGames } from '../lib/plural'
import { isStatus, SHELF_ORDER, STATUS, type Status } from '../lib/status'
import { useIsMobile, useMediaQuery } from '../lib/useMediaQuery'
import styles from './LibraryPage.module.css'

const SORTS: Array<[LibrarySort, string]> = [
  ['score', 'Twojej oceny'],
  ['title', 'Tytułu'],
  ['added', 'Daty dodania'],
]

/** Na telefonie grzbiety układamy w rzędy po tyle sztuk, każdy na swojej desce. */
const MOBILE_ROW = 7

function isSort(v: string | null): v is LibrarySort {
  return v === 'score' || v === 'title' || v === 'added'
}

function groupByStatus(entries: Entry[]) {
  const out = Object.fromEntries(SHELF_ORDER.map((s) => [s, [] as Entry[]])) as Record<Status, Entry[]>
  for (const e of entries) out[e.status].push(e)
  return out
}

function EntrySpine({ e, selected, onSelect }: { e: Entry; selected: boolean; onSelect: (id: number) => void }) {
  return (
    <Spine
      title={e.game.title}
      status={e.status}
      platform={e.platform}
      score={e.score}
      colors={e.game.spine}
      selected={selected}
      onSelect={() => onSelect(e.id)}
    />
  )
}

function EmptyShelf() {
  return (
    <div className={styles.empty}>
      <p>Na tej półce nic nie stoi.</p>
      <Link to="/search" className="btn btn-ghost">
        Dodaj grę
      </Link>
    </div>
  )
}

export function LibraryPage() {
  const [params, setParams] = useSearchParams()
  const sort: LibrarySort = isSort(params.get('sort')) ? (params.get('sort') as LibrarySort) : 'score'
  const view = params.get('view') === 'grid' ? 'grid' : 'shelf'
  const selectedId = Number(params.get('game')) || null
  const isMobile = useIsMobile()
  // Poniżej 900 px panel nie mieści się obok półek, więc wybrana gra pokazuje się w dolnym arkuszu.
  const isCompact = useMediaQuery('(max-width: 899.98px)')
  const lib = useLibrary(sort)

  useEffect(() => {
    document.title = 'Biblioteka · GameLogg'
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
  const select = (id: number) => update('game', String(id))

  if (lib.isPending) return <div className="page" aria-busy="true" />
  if (lib.isError) {
    return (
      <div className="page">
        <h1 className="page-title">Biblioteka</h1>
        <p role="alert" className="page-lead" style={{ color: 'var(--warn)' }}>
          {lib.error.message}
        </p>
      </div>
    )
  }
  if (lib.data.length === 0) return <Welcome />

  const entries = lib.data
  const shelves = groupByStatus(entries)
  const selected = entries.find((e) => e.id === selectedId) ?? null

  if (isMobile) {
    const shelfParam = params.get('shelf')
    const firstFilled = SHELF_ORDER.find((s) => shelves[s].length > 0) ?? 'in_progress'
    const current: Status = isStatus(shelfParam) ? shelfParam : (selected?.status ?? firstFilled)
    const items = shelves[current]
    const rows: Entry[][] = []
    for (let i = 0; i < items.length; i += MOBILE_ROW) rows.push(items.slice(i, i + MOBILE_ROW))

    return (
      <div className={`page ${styles.mobile}`}>
        <div className={styles.mobileHead}>
          <h1 className="page-title">Biblioteka</h1>
          <Link to="/search" className={styles.addIcon} aria-label="Dodaj grę">
            <PlusIcon />
          </Link>
        </div>

        <div role="group" aria-label="Półka" className={styles.chips}>
          {SHELF_ORDER.map((s) => (
            <button
              key={s}
              type="button"
              className={styles.chip}
              aria-pressed={s === current}
              onClick={() =>
                setParams(
                  (p) => {
                    p.set('shelf', s)
                    p.delete('game')
                    return p
                  },
                  { replace: true },
                )
              }
            >
              <span aria-hidden="true" className={styles.chipDot} style={{ background: STATUS[s].color }} />
              {STATUS[s].label}
              <span className={`${styles.chipCount} num`}>{shelves[s].length}</span>
            </button>
          ))}
        </div>

        <h2 className="sr-only">{STATUS[current].label}</h2>
        {items.length === 0 ? (
          <EmptyShelf />
        ) : (
          rows.map((row, i) => (
            <Shelf key={i}>
              {row.map((e) => (
                <EntrySpine key={e.id} e={e} selected={e.id === selectedId} onSelect={select} />
              ))}
            </Shelf>
          ))
        )}

        {selected && <div className={styles.sheetSpacer} />}
        {selected && <GameSheet entry={selected} onClose={() => update('game', null)} />}
      </div>
    )
  }

  return (
    <div className="page">
      <div className={styles.head}>
        <div>
          <h1 className="page-title">Biblioteka</h1>
          <p className="page-lead">
            {countGames(entries.length)} na sześciu półkach. Kliknij grzbiet, żeby zobaczyć grę.
          </p>
        </div>
        <div className={styles.controls}>
          <div role="group" aria-label="Widok" className={styles.seg}>
            <button type="button" aria-pressed={view === 'shelf'} onClick={() => update('view', null)}>
              Półka
            </button>
            <button type="button" aria-pressed={view === 'grid'} onClick={() => update('view', 'grid')}>
              Okładki
            </button>
          </div>
          <label className={styles.sortLabel}>
            Układaj według
            <select value={sort} onChange={(e) => update('sort', e.target.value === 'score' ? null : e.target.value)}>
              {SORTS.map(([v, label]) => (
                <option key={v} value={v}>
                  {label}
                </option>
              ))}
            </select>
          </label>
        </div>
      </div>

      <div className={styles.layout}>
        <section aria-label="Półki" className={styles.shelves}>
          {view === 'shelf' ? (
            SHELF_ORDER.map((s) => (
              <section key={s} aria-labelledby={`shelf-${s}`} className={styles.shelfBlock}>
                <div className={styles.shelfHead}>
                  <span aria-hidden="true" className={styles.shelfDot} style={{ background: STATUS[s].color }} />
                  <h2 id={`shelf-${s}`} className={styles.shelfTitle}>
                    {STATUS[s].label}
                  </h2>
                  <span className={`${styles.shelfCount} num`}>{shelves[s].length}</span>
                </div>
                {shelves[s].length === 0 ? (
                  <div className={styles.emptyRow}>
                    <p>Na tej półce nic nie stoi.</p>
                    <Board />
                  </div>
                ) : (
                  <Shelf>
                    {shelves[s].map((e) => (
                      <EntrySpine key={e.id} e={e} selected={e.id === selectedId} onSelect={select} />
                    ))}
                  </Shelf>
                )}
              </section>
            ))
          ) : (
            <ul className={styles.grid}>
              {entries.map((e) => (
                <li key={e.id}>
                  <button
                    type="button"
                    className={styles.tile}
                    aria-pressed={e.id === selectedId}
                    aria-label={`${e.game.title}, ${STATUS[e.status].label}${e.score != null ? `, ocena ${e.score}` : ''}`}
                    onClick={() => select(e.id)}
                  >
                    <Cover className={styles.tileCover} title={e.game.title} src={e.game.coverUrl} colors={e.game.spine} />
                    <span aria-hidden="true" className={styles.tileTitle}>
                      {e.game.title}
                    </span>
                    <span aria-hidden="true" className={styles.tileStatus}>
                      <span className={styles.tileDot} style={{ background: STATUS[e.status].color }} />
                      {STATUS[e.status].label}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>

        {!isCompact && <GamePanel entry={selected} />}
      </div>
      {isCompact && selected && <div className={styles.sheetSpacer} />}
      {isCompact && selected && <GameSheet entry={selected} onClose={() => update('game', null)} />}
    </div>
  )
}

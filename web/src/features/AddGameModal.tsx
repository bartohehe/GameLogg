import { useId, useState } from 'react'
import { Link } from 'react-router'
import { Cover } from '../components/Cover'
import { CloseButton, Modal } from '../components/Modal'
import { Board } from '../components/Shelf'
import { Spine } from '../components/Spine'
import { useAddGame, useRemoveEntry, type Entry, type Game } from '../lib/games'
import { ADD_ORDER, STATUS, type Status } from '../lib/status'
import styles from './AddGameModal.module.css'

interface Props {
  game: Pick<Game, 'igdbId' | 'title' | 'year' | 'developer' | 'coverUrl' | 'platforms'>
  onClose: () => void
  initialStatus?: Status
}

function meta(year: number | null, developer: string) {
  return [year, developer].filter(Boolean).join(', ')
}

export function AddGameModal({ game, onClose, initialStatus = 'planned' }: Props) {
  const titleId = useId()
  const [status, setStatus] = useState<Status>(initialStatus)
  const [platform, setPlatform] = useState(game.platforms[0] ?? '')
  const [placed, setPlaced] = useState<Entry | null>(null)
  const add = useAddGame()
  const remove = useRemoveEntry()

  function submit() {
    add.mutate({ igdbId: game.igdbId, status, platform }, { onSuccess: (entry) => setPlaced(entry) })
  }

  function undo() {
    if (!placed) return
    remove.mutate(placed.id, {
      onSuccess: () => {
        setPlaced(null)
        add.reset()
      },
    })
  }

  return (
    <Modal labelledBy={titleId} onClose={onClose}>
      {!placed ? (
        <div>
          <div className={styles.head}>
            <Cover className={styles.thumb} title={game.title} src={game.coverUrl} showTitle={false} />
            <div className={styles.headText}>
              <h1 id={titleId} className={styles.title}>
                {game.title}
              </h1>
              {meta(game.year, game.developer) && <p className={styles.meta}>{meta(game.year, game.developer)}</p>}
            </div>
            <CloseButton onClick={onClose} />
          </div>

          <section className={styles.section} aria-labelledby={`${titleId}-shelf`}>
            <h2 id={`${titleId}-shelf`} className={styles.h2}>
              Na którą półkę?
            </h2>
            <div role="group" aria-labelledby={`${titleId}-shelf`} className={styles.shelves}>
              {ADD_ORDER.map((s) => (
                <button
                  key={s}
                  type="button"
                  className={styles.shelf}
                  aria-pressed={status === s}
                  onClick={() => setStatus(s)}
                >
                  <span aria-hidden="true" className={styles.band} style={{ background: STATUS[s].color }} />
                  <span className={styles.shelfText}>
                    <span className={styles.shelfLabel}>{STATUS[s].label}</span>
                    <span className={styles.shelfHint}>{STATUS[s].hint}</span>
                  </span>
                </button>
              ))}
            </div>
          </section>

          {game.platforms.length > 0 && (
            <section className={styles.section} aria-labelledby={`${titleId}-pl`}>
              <h2 id={`${titleId}-pl`} className={styles.h2}>
                Na czym grasz?
              </h2>
              <div role="group" aria-labelledby={`${titleId}-pl`} className={styles.platforms}>
                {game.platforms.map((p) => (
                  <button
                    key={p}
                    type="button"
                    className={styles.platform}
                    aria-pressed={platform === p}
                    onClick={() => setPlatform(p)}
                  >
                    {p}
                  </button>
                ))}
              </div>
              <p className={styles.note}>Pokazuję tylko platformy, na które gra wyszła.</p>
            </section>
          )}

          <p role="alert" className={styles.error}>
            {add.error?.message ?? ''}
          </p>

          <div className={styles.footer}>
            <button type="button" className="btn btn-ghost" onClick={onClose}>
              Anuluj
            </button>
            <button type="button" className="btn btn-primary" aria-busy={add.isPending} onClick={submit}>
              {add.isPending ? 'Stawiam na półce…' : `Postaw na półce ${STATUS[status].label}`}
            </button>
          </div>
        </div>
      ) : (
        <div className={styles.placed}>
          <div className={styles.placedTop}>
            <CloseButton onClick={onClose} />
          </div>
          <div aria-hidden="true" inert className={styles.mini}>
            <div className={styles.miniRow}>
              <span className={styles.ghost} style={{ height: 150 }} />
              <span className={styles.ghost} style={{ height: 160 }} />
              <span className={styles.lifted}>
                <Spine
                  title={placed.game.title}
                  status={placed.status}
                  platform={placed.platform}
                  colors={placed.game.spine}
                />
              </span>
              <span className={styles.ghost} style={{ height: 146 }} />
            </div>
            <Board />
          </div>
          <h1 id={titleId} className={styles.placedTitle}>
            {placed.game.title} stoi na półce {STATUS[placed.status].label}
          </h1>
          <p className={styles.meta}>
            {placed.platform ? `Platforma: ${placed.platform}. ` : ''}Zmienisz to w każdej chwili na karcie gry.
          </p>
          <p role="alert" className={styles.error}>
            {remove.error?.message ?? ''}
          </p>
          <div className={styles.placedActions}>
            <button type="button" className="btn btn-ghost" aria-busy={remove.isPending} onClick={undo}>
              Cofnij
            </button>
            <Link to={`/library?game=${placed.id}`} className="btn btn-primary" onClick={onClose}>
              Zobacz na półce
            </Link>
          </div>
        </div>
      )}
    </Modal>
  )
}

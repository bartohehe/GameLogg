import { Link } from 'react-router'
import { Cover } from '../components/Cover'
import { CloseButton } from '../components/Modal'
import { CategoryBreakdown } from '../components/CategoryBreakdown'
import { useEntryRankings, type Entry } from '../lib/games'
import { STATUS } from '../lib/status'
import styles from './GamePanel.module.css'
import { targetFromEntry, useOpenRanking } from './ranking/context'

function meta(e: Entry) {
  return [e.game.developer, e.game.year].filter(Boolean).join(', ')
}

function StatusPill({ entry }: { entry: Entry }) {
  return (
    <p className={styles.pill}>
      <span aria-hidden="true" className={styles.dot} style={{ background: STATUS[entry.status].color }} />
      {STATUS[entry.status].label}
      {entry.platform ? ` na ${entry.platform}` : ''}
    </p>
  )
}

/** Boczny panel wybranej gry w bibliotece (desktop). */
export function GamePanel({ entry }: { entry: Entry | null }) {
  if (!entry) {
    return (
      <aside aria-label="Wybrana gra" className={`${styles.panel} ${styles.empty}`}>
        <p>Kliknij grzbiet, żeby zobaczyć grę.</p>
      </aside>
    )
  }
  return <PanelBody entry={entry} />
}

function PanelBody({ entry }: { entry: Entry }) {
  const openRanking = useOpenRanking()
  const completed = entry.status === 'completed'
  const places = useEntryRankings(entry.id, completed)
  const rateLabel = entry.score != null ? 'Zmień miejsca w rankingu' : 'Oceń w rankingu'

  return (
    <aside aria-label="Wybrana gra" className={styles.panel}>
      <div className={styles.top}>
        <Cover className={styles.cover} title={entry.game.title} src={entry.game.coverUrl} colors={entry.game.spine} />
        <div className={styles.topText}>
          <h2 className={styles.title}>{entry.game.title}</h2>
          {meta(entry) && <p className={styles.meta}>{meta(entry)}</p>}
          <StatusPill entry={entry} />
        </div>
      </div>

      <div className={styles.section}>
        {entry.score != null ? (
          <>
            <div className={styles.scoreRow}>
              <h3 className={styles.h3}>Twoja ocena</h3>
              <span className={`${styles.score} num`}>{entry.score}</span>
            </div>
            <p className={styles.muted}>Liczona z miejsc w twoich rankingach kategorii.</p>
            {places.data && <CategoryBreakdown categories={places.data.categories} />}
          </>
        ) : (
          <>
            <h3 className={styles.h3}>Jeszcze bez oceny</h3>
            <p className={styles.muted}>
              {entry.status === 'completed'
                ? 'Ustaw grę w sześciu rankingach, a ocena policzy się sama.'
                : 'Gdy skończysz, ustawisz grę w sześciu rankingach, a ocena policzy się sama.'}
            </p>
          </>
        )}
      </div>

      <div className={styles.section}>
        <h3 className={styles.h3}>Notatka</h3>
        <p className={entry.note ? styles.note : styles.muted}>
          {entry.note || 'Brak notatki. Dopisz, co zapamiętałeś z tej gry.'}
        </p>
      </div>

      <div className={styles.actions}>
        {completed && (
          <button type="button" className="btn btn-primary" onClick={() => openRanking(targetFromEntry(entry))}>
            {rateLabel}
          </button>
        )}
        <Link to={`/game/${entry.game.igdbId}`} className={completed ? 'btn btn-ghost' : 'btn btn-primary'}>
          Otwórz kartę gry
        </Link>
      </div>
    </aside>
  )
}

/** Dolny arkusz z wybraną grą (telefon). */
export function GameSheet({ entry, onClose }: { entry: Entry; onClose: () => void }) {
  return (
    <aside aria-label="Wybrana gra" className={styles.sheet}>
      <div aria-hidden="true" className={styles.grip} />
      <div className={styles.sheetRow}>
        <Cover className={styles.sheetThumb} title={entry.game.title} src={entry.game.coverUrl} colors={entry.game.spine} showTitle={false} />
        <div className={styles.sheetText}>
          <h2 className={styles.sheetTitle}>{entry.game.title}</h2>
          <p className={styles.sheetMeta}>
            <span aria-hidden="true" className={styles.dot} style={{ background: STATUS[entry.status].color }} />
            {STATUS[entry.status].label}
            {entry.platform ? ` na ${entry.platform}` : ''}
          </p>
        </div>
        {entry.score != null && (
          <span className={`${styles.sheetScore} num`} aria-label={`Ocena ${entry.score}`}>
            {entry.score}
          </span>
        )}
        <CloseButton onClick={onClose} />
      </div>
      <Link to={`/game/${entry.game.igdbId}`} className={`btn btn-ghost ${styles.sheetLink}`}>
        Otwórz kartę gry
      </Link>
    </aside>
  )
}

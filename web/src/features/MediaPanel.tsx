import { useEffect, useId, useRef, useState } from 'react'
import { Cover } from '../components/Cover'
import { CloseButton } from '../components/Modal'
import {
  lengthText,
  MEDIA_STATUS,
  MEDIA_STATUS_ORDER,
  TYPE_LABEL,
  useEpisode,
  useRemoveMedia,
  useUpdateMedia,
  type MediaItem,
} from '../lib/media'
import styles from './MediaPanel.module.css'

function meta(m: MediaItem) {
  return [TYPE_LABEL[m.type], m.year, m.creator].filter(Boolean).join(', ')
}

/** Panel wybranego tytułu: status, odcinki, ręczna ocena 0–100, notatka. */
export function MediaPanel({ item, onClose, variant }: { item: MediaItem; onClose: () => void; variant: 'aside' | 'sheet' }) {
  return (
    <aside aria-label="Wybrany tytuł" className={variant === 'sheet' ? styles.sheet : styles.panel}>
      <div className={styles.top}>
        <Cover className={styles.cover} title={item.title} src={item.coverUrl} colors={item.spine} showTitle={false} />
        <div className={styles.topText}>
          <h2 className={styles.title}>{item.title}</h2>
          {meta(item) && <p className={styles.meta}>{meta(item)}</p>}
          {lengthText(item) && <p className={styles.meta}>{lengthText(item)}</p>}
        </div>
        <CloseButton onClick={onClose} />
      </div>
      {/* key: po zmianie tytułu formularze zaczynają od jego wartości */}
      <PanelForms key={item.id} item={item} onRemoved={onClose} />
    </aside>
  )
}

function PanelForms({ item, onRemoved }: { item: MediaItem; onRemoved: () => void }) {
  const update = useUpdateMedia()
  const episode = useEpisode()
  const remove = useRemoveMedia()
  const scoreId = useId()
  const noteId = useId()
  const [score, setScore] = useState(item.score != null ? String(item.score) : '')
  const [scoreError, setScoreError] = useState<string | null>(null)
  const [confirming, setConfirming] = useState(false)
  const episodic = item.type !== 'film'

  function saveScore() {
    const raw = score.trim()
    const value = raw === '' ? null : Number(raw)
    if (value != null && (!Number.isInteger(value) || value < 0 || value > 100)) {
      setScoreError('Ocena to liczba całkowita od 0 do 100.')
      return
    }
    setScoreError(null)
    if (value === item.score) return
    update.mutate({ id: item.id, patch: { score: value } })
  }

  const error = update.error ?? episode.error ?? remove.error

  return (
    <>
      <section className={styles.section} aria-labelledby={`${scoreId}-st`}>
        <h3 id={`${scoreId}-st`} className={styles.h3}>
          Status
        </h3>
        <div role="group" aria-labelledby={`${scoreId}-st`} className={styles.statuses}>
          {MEDIA_STATUS_ORDER.map((s) => (
            <button
              key={s}
              type="button"
              className={styles.status}
              aria-pressed={item.status === s}
              disabled={update.isPending}
              onClick={() => s !== item.status && update.mutate({ id: item.id, patch: { status: s } })}
            >
              <span aria-hidden="true" className={styles.dot} style={{ background: MEDIA_STATUS[s].color }} />
              {MEDIA_STATUS[s].label}
            </button>
          ))}
        </div>

        {episodic && (
          <div className={styles.episodes}>
            <span className="num">
              {item.episodes != null
                ? `Odcinek ${item.watchedEpisodes ?? 0} z ${item.episodes}`
                : `Obejrzane odcinki: ${item.watchedEpisodes ?? 0}`}
            </span>
            <button
              type="button"
              className="btn btn-ghost"
              aria-busy={episode.isPending}
              disabled={item.episodes != null && (item.watchedEpisodes ?? 0) >= item.episodes}
              onClick={() => episode.mutate(item.id)}
            >
              +1 odcinek
            </button>
          </div>
        )}
      </section>

      <section className={styles.section}>
        <label htmlFor={scoreId} className={styles.h3}>
          Twoja ocena (0–100)
        </label>
        <div className={styles.scoreRow}>
          <input
            id={scoreId}
            className={`${styles.scoreInput} num`}
            type="number"
            inputMode="numeric"
            min={0}
            max={100}
            step={1}
            value={score}
            placeholder="–"
            aria-invalid={scoreError ? true : undefined}
            aria-describedby={`${scoreId}-hint`}
            onChange={(e) => setScore(e.target.value)}
            onBlur={saveScore}
            onKeyDown={(e) => e.key === 'Enter' && saveScore()}
          />
          <p id={`${scoreId}-hint`} className={scoreError ? styles.error : styles.muted}>
            {scoreError ?? 'Wpisz liczbę i wyjdź z pola albo naciśnij Enter. Puste pole usuwa ocenę.'}
          </p>
        </div>
      </section>

      <NoteField item={item} id={noteId} />

      {error && (
        <p role="alert" className={styles.error}>
          {error.message}
        </p>
      )}

      <section className={styles.remove}>
        {!confirming ? (
          <button type="button" className={styles.removeBtn} onClick={() => setConfirming(true)}>
            Usuń z listy
          </button>
        ) : (
          <div role="alert" className={styles.confirm}>
            <p>{item.title} zniknie z listy razem z oceną i notatką.</p>
            <div className={styles.confirmActions}>
              <button type="button" className="btn btn-primary" onClick={() => remove.mutate(item.id, { onSuccess: onRemoved })}>
                Usuń z listy
              </button>
              <button type="button" className="btn btn-ghost" onClick={() => setConfirming(false)}>
                Zostaw
              </button>
            </div>
          </div>
        )}
      </section>
    </>
  )
}

/** Notatka z autozapisem 800 ms po ostatnim znaku. */
function NoteField({ item, id }: { item: MediaItem; id: string }) {
  const update = useUpdateMedia()
  const [text, setText] = useState(item.note)
  const [state, setState] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle')
  const saved = useRef(item.note)
  const mutate = useRef(update.mutate)

  useEffect(() => {
    mutate.current = update.mutate
  })

  useEffect(() => {
    if (text === saved.current) return
    const t = setTimeout(() => {
      setState('saving')
      mutate.current(
        { id: item.id, patch: { note: text } },
        {
          onSuccess: () => {
            saved.current = text
            setState('saved')
          },
          onError: () => setState('error'),
        },
      )
    }, 800)
    return () => clearTimeout(t)
  }, [text, item.id])

  return (
    <section className={styles.section}>
      <label htmlFor={id} className={styles.h3}>
        Notatka
      </label>
      <textarea
        id={id}
        rows={3}
        maxLength={5000}
        value={text}
        placeholder="Co zapamiętałeś?"
        aria-describedby={`${id}-status`}
        onChange={(e) => setText(e.target.value)}
      />
      <p id={`${id}-status`} role="status" className={state === 'error' ? styles.error : styles.muted}>
        {state === 'saving' ? 'Zapisuję…' : state === 'saved' ? 'Zapisano.' : state === 'error' ? 'Nie zapisano notatki.' : ''}
      </p>
    </section>
  )
}

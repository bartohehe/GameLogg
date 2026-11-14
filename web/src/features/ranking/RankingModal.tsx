import { useQueries } from '@tanstack/react-query'
import { useEffect, useId, useRef, useState } from 'react'
import { Cover } from '../../components/Cover'
import { CloseButton, Modal } from '../../components/Modal'
import { rankingKey, rankingsApi, useInvalidateLibrary, type RankedItem } from '../../lib/games'
import { CATEGORIES, CATEGORY_LABEL, insert, points, score, type Category, type Place } from '../../lib/ranking'
import type { RankingTarget } from './context'
import styles from './RankingModal.module.css'

interface Props {
  target: RankingTarget
  onClose: () => void
}

/** Okno oceny: w każdej z sześciu kategorii wstawiasz grę między inne ukończone gry. */
export function RankingModal({ target, onClose }: Props) {
  const titleId = useId()
  const results = useQueries({
    queries: CATEGORIES.map((c) => ({ queryKey: rankingKey(c), queryFn: () => rankingsApi.list(c), staleTime: 0 })),
  })
  const error = results.find((r) => r.isError)?.error
  const ready = results.every((r) => r.isSuccess)

  return (
    <Modal labelledBy={titleId} onClose={onClose} width={1060}>
      <div className={styles.shell}>
        <div className={styles.header}>
          <div>
            <h1 id={titleId} className={styles.title}>
              Gdzie stawiasz {target.title}?
            </h1>
            <p className={styles.lead}>
              Wstaw grę między dwie inne w każdym z sześciu rankingów. Ocena wyliczy się z miejsc.
            </p>
          </div>
          <CloseButton onClick={onClose} />
        </div>
        {error ? (
          <p role="alert" className={styles.loadError}>
            {error.message}
          </p>
        ) : !ready ? (
          <p role="status" className={styles.loading}>
            Wczytuję rankingi…
          </p>
        ) : (
          <Editor
            target={target}
            lists={Object.fromEntries(CATEGORIES.map((c, i) => [c, results[i]!.data!])) as Record<Category, RankedItem[]>}
            onDone={onClose}
          />
        )}
      </div>
    </Modal>
  )
}

type Slots = Record<Category, number | null>

interface EditorProps {
  target: RankingTarget
  lists: Record<Category, RankedItem[]>
  onDone: () => void
}

function Editor({ target, lists: rawLists, onDone }: EditorProps) {
  const invalidate = useInvalidateLibrary()

  // Listy bez ocenianej gry; jej dotychczasowe miejsce to wstępnie wybrana szczelina.
  const [lists] = useState(() => {
    const out = {} as Record<Category, RankedItem[]>
    for (const c of CATEGORIES) out[c] = rawLists[c].filter((r) => r.entry.id !== target.entryId)
    return out
  })
  // Miejsca sprzed otwarcia okna: od nich zależy etykieta „nowa” albo „przeniesiona”.
  const [initial] = useState<Slots>(() => {
    const out = {} as Slots
    for (const c of CATEGORIES) {
      const i = rawLists[c].findIndex((r) => r.entry.id === target.entryId)
      out[c] = i >= 0 ? i : null
    }
    return out
  })
  const [slots, setSlots] = useState<Slots>(initial)
  const [step, setStep] = useState(0)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  // Po wstawieniu focus idzie na „Cofnij”, po cofnięciu na szczelinę, po zmianie kroku na nagłówek.
  const [focusTarget, setFocusTarget] = useState<{ kind: 'undo' } | { kind: 'slot'; index: number } | { kind: 'heading' } | null>(null)

  const cat = CATEGORIES[step]!
  const list = lists[cat]
  const slot = slots[cat]
  const last = step === CATEGORIES.length - 1

  const undoRef = useRef<HTMLButtonElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const slotRefs = useRef(new Map<number, HTMLButtonElement>())

  useEffect(() => {
    if (!focusTarget) return
    if (focusTarget.kind === 'undo') {
      undoRef.current?.focus()
      undoRef.current?.scrollIntoView({ block: 'nearest' })
    } else if (focusTarget.kind === 'slot') {
      slotRefs.current.get(focusTarget.index)?.focus()
    } else {
      headingRef.current?.focus()
    }
  }, [focusTarget])

  const placeOf = (c: Category): Place | undefined =>
    slots[c] == null ? undefined : { pos: slots[c]! + 1, n: lists[c].length + 1 }

  const places: Partial<Record<Category, Place>> = {}
  for (const c of CATEGORIES) {
    const p = placeOf(c)
    if (p) places[c] = p
  }
  const total = score(places)
  const placedCount = Object.keys(places).length
  const current = placeOf(cat)

  function setSlot(c: Category, value: number | null) {
    setSlots((s) => ({ ...s, [c]: value }))
  }

  function place(index: number) {
    setSlot(cat, index)
    setFocusTarget({ kind: 'undo' })
  }

  function undo() {
    const index = slot ?? 0
    setSlot(cat, null)
    setFocusTarget({ kind: 'slot', index })
  }

  function goTo(i: number) {
    setStep(i)
    setFocusTarget({ kind: 'heading' })
  }

  function skip() {
    setSlot(cat, null)
    if (!last) goTo(step + 1)
  }

  async function save() {
    setSaving(true)
    setSaveError(null)
    try {
      // Każda kategoria to pełna kolejność; pominięta kategoria usuwa grę z rankingu.
      for (const c of CATEGORIES) {
        const ids = lists[c].map((r) => r.entry.id)
        const s = slots[c]
        await rankingsApi.put(c, s == null ? ids : insert(ids, target.entryId, s))
      }
      await invalidate()
      onDone()
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Nie udało się zapisać oceny.')
      setSaving(false)
    }
  }

  const rows: React.ReactNode[] = []
  for (let i = 0; i <= list.length; i++) {
    if (slot === i) {
      rows.push(
        <li key="new" className={styles.newRow}>
          <span className={`${styles.rank} ${styles.rankNew} num`}>{i + 1}</span>
          <span
            className={`${styles.bar} ${styles.barNew}`}
            style={{
              color: target.spine.ink,
              background: `linear-gradient(90deg, ${target.spine.color} 0 88%, ${target.spine.accent} 88% 100%)`,
            }}
          >
            <span className={styles.barTitle}>{target.title}</span>
            <span className={styles.newTag}>
              {initial[cat] == null ? 'nowa' : initial[cat] === i ? 'bez zmian' : 'przeniesiona'}
            </span>
          </span>
          <button ref={undoRef} type="button" className={styles.undo} onClick={undo}>
            Cofnij<span className="sr-only">: {target.title} z miejsca {i + 1}</span>
          </button>
        </li>,
      )
    } else {
      const above = i > 0 ? list[i - 1]!.entry.game.title : null
      const below = i < list.length ? list[i]!.entry.game.title : null
      // Nazwa dostępna zaczyna się od widocznego tekstu „Wstaw tutaj” (WCAG 2.5.3).
      const label = above
        ? `Wstaw tutaj: pod ${above}${below ? `, nad ${below}` : ''}`
        : below
          ? `Wstaw tutaj: na pierwsze miejsce, nad ${below}`
          : 'Wstaw tutaj: jako pierwszą grę w rankingu'
      rows.push(
        <li key={`slot-${i}`}>
          <button
            ref={(el) => {
              if (el) slotRefs.current.set(i, el)
              else slotRefs.current.delete(i)
            }}
            type="button"
            className={styles.slot}
            aria-label={label}
            onClick={() => place(i)}
          >
            <span className={styles.line} />
            <span aria-hidden="true">Wstaw tutaj</span>
            <span className={styles.line} />
          </button>
        </li>,
      )
    }
    if (i < list.length) {
      const e = list[i]!.entry
      const rank = slot != null && slot <= i ? i + 2 : i + 1
      rows.push(
        <li key={e.id} className={styles.gameRow}>
          <span className={`${styles.rank} num`}>{rank}</span>
          <span
            className={styles.bar}
            style={{
              color: e.game.spine.ink,
              background: `linear-gradient(90deg, ${e.game.spine.color} 0 88%, ${e.game.spine.accent} 88% 100%)`,
            }}
          >
            <span className={styles.barTitle}>{e.game.title}</span>
          </span>
        </li>,
      )
    }
  }

  const label = CATEGORY_LABEL[cat]
  const meta = [target.developer, target.year].filter(Boolean).join(', ')

  return (
    <>
      <nav aria-label="Kategorie" className={styles.steps}>
        <ol>
          {CATEGORIES.map((c, i) => {
            const done = slots[c] != null
            return (
              <li key={c}>
                <button
                  type="button"
                  className={styles.step}
                  aria-current={i === step ? 'step' : undefined}
                  onClick={() => goTo(i)}
                >
                  <span
                    className={`${styles.badge} ${i === step ? styles.badgeCurrent : done ? styles.badgeDone : ''}`}
                    aria-hidden="true"
                  >
                    {i + 1}
                  </span>
                  {CATEGORY_LABEL[c]}
                  <span className="sr-only">{done ? ', ustawiona' : ', bez miejsca'}</span>
                </button>
              </li>
            )
          })}
        </ol>
      </nav>

      <div className={styles.body}>
        <div className={styles.listCol}>
          <div className={styles.listHead}>
            <h2 ref={headingRef} tabIndex={-1} className={styles.listTitle}>
              Ranking: {label}
            </h2>
            <span className={styles.listHint}>Najlepsza na górze</span>
          </div>
          <ol className={styles.list}>{rows}</ol>
        </div>

        <aside className={styles.side} aria-label="Ocena">
          <div className={styles.target}>
            <Cover className={styles.thumb} title={target.title} src={target.coverUrl} colors={target.spine} showTitle={false} />
            <div>
              <div className={styles.targetTitle}>{target.title}</div>
              {meta && <div className={styles.targetMeta}>{meta}</div>}
            </div>
          </div>
          <div className={styles.scoreBlock} aria-live="polite">
            <div className={styles.muted}>{label}</div>
            <div className={`${styles.catPts} num`}>{current ? `${Math.round(points(current.pos, current.n))} pkt` : '–'}</div>
            <div className={styles.hint}>
              {current
                ? `${current.pos}. miejsce z ${current.n}`
                : `Kliknij linię między grami, żeby wstawić ${target.title}.`}
            </div>
          </div>
          <div className={styles.scoreBlock} aria-live="polite">
            <div className={styles.muted}>Ocena końcowa</div>
            <div className={`${styles.total} num`}>{total ?? '–'}</div>
            <div className={styles.hint}>
              {placedCount === 0
                ? 'Pojawi się po pierwszej kategorii.'
                : placedCount === CATEGORIES.length
                  ? 'Ze wszystkich sześciu kategorii.'
                  : `Z ${placedCount} na 6 kategorii. Zmieni się, gdy ustawisz resztę.`}
            </div>
          </div>
          <div className={styles.actions}>
            {saveError && (
              <p role="alert" className={styles.saveError}>
                {saveError}
              </p>
            )}
            <button
              type="button"
              className={`btn btn-primary ${styles.next}`}
              aria-busy={saving}
              disabled={saving}
              onClick={() => (last ? save() : goTo(step + 1))}
            >
              {last ? (saving ? 'Zapisuję…' : 'Zapisz ocenę') : `Dalej: ${CATEGORY_LABEL[CATEGORIES[step + 1]!]}`}
            </button>
            <button type="button" className="btn btn-ghost" onClick={skip} disabled={saving}>
              Pomiń tę kategorię
            </button>
          </div>
        </aside>
      </div>
    </>
  )
}

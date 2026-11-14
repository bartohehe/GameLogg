import type { CategoryPlace } from '../lib/games'
import { CATEGORY_LABEL } from '../lib/ranking'
import styles from './CategoryBreakdown.module.css'

/** Powyżej tylu gier kreski byłyby za cienkie: rysujemy pasek ze znacznikiem. */
const MAX_TICKS = 40

function Ticks({ position, n, tall }: { position: number; n: number; tall?: boolean }) {
  if (n > MAX_TICKS) {
    const left = n <= 1 ? 0 : ((position - 1) / (n - 1)) * 100
    return (
      <span aria-hidden="true" className={`${styles.track} ${tall ? styles.trackTall : ''}`}>
        <span className={styles.marker} style={{ left: `calc(${left}% - 3px)` }} />
      </span>
    )
  }
  return (
    <span aria-hidden="true" className={`${styles.ticks} ${tall ? styles.ticksTall : ''}`}>
      {Array.from({ length: n }, (_, i) => {
        const on = i + 1 === position
        // Lekko różne wysokości dają rytm grzbietów na półce (jak w mockupie).
        const h = tall ? (on ? 44 : 26 + ((((i + 1) * 7) % 5) * 3)) : undefined
        return <span key={i} className={on ? `${styles.tick} ${styles.on}` : styles.tick} style={h ? { height: h } : undefined} />
      })}
    </span>
  )
}

/** Miejsca gry w sześciu kategoriach. Wariant „full” to karta gry, „compact” to panel w bibliotece. */
export function CategoryBreakdown({ categories, variant = 'compact' }: { categories: CategoryPlace[]; variant?: 'compact' | 'full' }) {
  if (variant === 'compact') {
    return (
      <ul className={styles.compact}>
        {categories.map((c) => (
          <li key={c.category}>
            <div className={styles.compactHead}>
              <span>{CATEGORY_LABEL[c.category]}</span>
              <span className={`${styles.muted} num`}>
                {c.position != null ? `${c.position}. z ${c.n}` : 'pominięta'}
              </span>
            </div>
            {c.position != null && <Ticks position={c.position} n={c.n} />}
          </li>
        ))}
      </ul>
    )
  }

  return (
    <ul className={styles.full}>
      {categories.map((c) => (
        <li key={c.category} className={styles.fullRow}>
          <div>
            <div className={styles.fullLabel}>{CATEGORY_LABEL[c.category]}</div>
            <div className={styles.muted}>waga {c.weight}%</div>
          </div>
          <div className={styles.fullTicks}>
            {c.position != null ? <Ticks position={c.position} n={c.n} tall /> : <span className={styles.skipped}>Pominięta w tej kategorii</span>}
          </div>
          <div className={styles.fullRight}>
            <div className={`${styles.pts} num`}>{c.points != null ? Math.round(c.points) : '–'}</div>
            <div className={`${styles.muted} num`}>{c.position != null ? `${c.position}. z ${c.n}` : 'bez miejsca'}</div>
          </div>
        </li>
      ))}
    </ul>
  )
}

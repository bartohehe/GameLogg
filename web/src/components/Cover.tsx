import { useState } from 'react'
import type { SpineColors } from '../lib/games'
import styles from './Cover.module.css'

interface Props {
  title: string
  src: string | null
  /** Kolory grzbietu na zaślepkę, gdy okładki brak albo się nie wczyta. */
  colors?: SpineColors | null
  className?: string
  /** Napis na zaślepce; dla miniatur można go wyłączyć. */
  showTitle?: boolean
}

/**
 * Okładka gry. Obraz jest dekoracyjny (alt=""), bo tytuł zawsze stoi obok.
 * Bez okładki rysujemy zaślepkę w kolorach grzbietu z tytułem.
 */
export function Cover({ title, src, colors, className, showTitle = true }: Props) {
  const [failed, setFailed] = useState(false)
  const cls = `${styles.cover} ${className ?? ''}`

  if (src && !failed) {
    return <img className={cls} src={src} alt="" loading="lazy" decoding="async" onError={() => setFailed(true)} />
  }
  const bg = colors ?? { color: '#2C2937', accent: '#46415A', ink: '#EEEAF4' }
  return (
    <div
      aria-hidden="true"
      className={`${cls} ${styles.placeholder}`}
      style={{
        background: `linear-gradient(160deg, ${bg.color} 0 58%, ${bg.accent} 58% 100%)`,
        color: bg.ink,
      }}
    >
      {showTitle && <span>{title}</span>}
    </div>
  )
}

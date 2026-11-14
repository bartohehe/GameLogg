import type { CSSProperties } from 'react'
import { Link } from 'react-router'
import type { SpineColors } from '../lib/games'
import { gameSpineSize, platformShort, type SpineSize } from '../lib/platforms'
import { spineAriaLabel } from '../lib/spineLabel'
import { STATUS, type Status } from '../lib/status'
import styles from './Spine.module.css'

export interface SpineProps {
  title: string
  /** Status gry. Grzbiety mediów podają zamiast niego statusLabel i bandColor. */
  status?: Status
  statusLabel?: string
  bandColor?: string
  /** Etykieta platformy („PS5”, „Xbox Series”). Pusta = bez skrótu, wymiary jak PC. */
  platform: string
  score?: number | null
  colors: SpineColors
  /** Zaznaczony grzbiet podnosi się o 14 px i ma miętową kreskę. */
  selected?: boolean
  onSelect?: () => void
  /** Własne wymiary (np. grzbiety filmów i seriali). Domyślnie zależą od platformy. */
  size?: SpineSize
  /** Krótki tekst pod paskiem statusu; domyślnie skrót platformy. */
  label?: string
  /** Adres: grzbiet staje się odnośnikiem zamiast przycisku. */
  href?: string
}

export function Spine({
  title,
  status,
  statusLabel,
  bandColor,
  platform,
  score,
  colors,
  selected,
  onSelect,
  size,
  label,
  href,
}: SpineProps) {
  const stLabel = statusLabel ?? (status ? STATUS[status].label : '')
  const band = bandColor ?? (status ? STATUS[status].color : 'var(--line-strong)')
  const { width, height } = size ?? gameSpineSize(platform)
  const style = {
    '--spine-w': `${width}px`,
    '--spine-h': `${height}px`,
    '--spine-color': colors.color,
    '--spine-accent': colors.accent,
    '--spine-ink': colors.ink,
    '--spine-band': band,
  } as CSSProperties

  const content = (
    <>
      <span aria-hidden="true" className={styles.band} />
      <span aria-hidden="true" className={styles.platform}>
        {label ?? (platform ? platformShort(platform) : '')}
      </span>
      <span aria-hidden="true" className={styles.title}>
        {title}
      </span>
      <span aria-hidden="true" className={`${styles.score} num`}>
        {score ?? ''}
      </span>
    </>
  )

  // Grzbiet-odnośnik (np. mini-półki na Odkryj i w profilu) zamiast przycisku z zaznaczeniem.
  if (href) {
    return (
      <Link to={href} className={styles.spine} style={style} aria-label={spineAriaLabel(title, stLabel, score)}>
        {content}
      </Link>
    )
  }

  return (
    <button
      type="button"
      className={styles.spine}
      style={style}
      aria-label={spineAriaLabel(title, stLabel, score)}
      aria-pressed={selected ?? false}
      onClick={onSelect}
    >
      {content}
    </button>
  )
}

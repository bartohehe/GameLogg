import type { CSSProperties } from 'react'
import type { SpineColors } from '../lib/games'
import { gameSpineSize, platformShort, type SpineSize } from '../lib/platforms'
import { spineAriaLabel } from '../lib/spineLabel'
import { STATUS, type Status } from '../lib/status'
import styles from './Spine.module.css'

export interface SpineProps {
  title: string
  status: Status
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
}

export function Spine({ title, status, platform, score, colors, selected, onSelect, size, label }: SpineProps) {
  const { width, height } = size ?? gameSpineSize(platform)
  const style = {
    '--spine-w': `${width}px`,
    '--spine-h': `${height}px`,
    '--spine-color': colors.color,
    '--spine-accent': colors.accent,
    '--spine-ink': colors.ink,
    '--spine-band': STATUS[status].color,
  } as CSSProperties

  return (
    <button
      type="button"
      className={styles.spine}
      style={style}
      aria-label={spineAriaLabel(title, status, score)}
      aria-pressed={selected ?? false}
      onClick={onSelect}
    >
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
    </button>
  )
}

import type { ReactNode } from 'react'
import styles from './Shelf.module.css'

/** Rząd grzbietów stojących na desce. Przewija się poziomo, gdy się nie mieści. */
export function Shelf({ children, label }: { children: ReactNode; label?: string }) {
  return (
    <div className={styles.shelf}>
      <div className={styles.row} role={label ? 'group' : undefined} aria-label={label}>
        {children}
      </div>
      <Board />
    </div>
  )
}

/** Deska półki: 12 px w --line z jaśniejszą krawędzią u góry. */
export function Board({ thin }: { thin?: boolean }) {
  return <div aria-hidden="true" className={thin ? `${styles.board} ${styles.thin}` : styles.board} />
}

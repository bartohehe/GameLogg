import { useEffect, useRef, type ReactNode } from 'react'
import { CloseIcon } from './icons'
import styles from './Modal.module.css'

interface Props {
  /** Id nagłówka, który nazywa okno. */
  labelledBy: string
  onClose: () => void
  children: ReactNode
  width?: number
}

/**
 * Okno modalne na natywnym <dialog> otwieranym przez showModal(): przeglądarka
 * blokuje resztę strony (focus nie wychodzi poza okno) i obsługuje Escape.
 * Po zamknięciu oddajemy focus elementowi, który okno otworzył.
 * Komponent montuje się otwarty; zamknięcie = odmontowanie przez rodzica.
 */
export function Modal({ labelledBy, onClose, children, width = 760 }: Props) {
  const ref = useRef<HTMLDialogElement>(null)
  const onCloseRef = useRef(onClose)

  useEffect(() => {
    onCloseRef.current = onClose
  })

  useEffect(() => {
    const dialog = ref.current
    if (!dialog) return
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
    dialog.showModal()
    return () => {
      if (dialog.open) dialog.close()
      if (opener?.isConnected) opener.focus()
    }
  }, [])

  return (
    <dialog
      ref={ref}
      className={styles.dialog}
      style={{ width: `min(${width}px, calc(100vw - 24px))` }}
      aria-labelledby={labelledBy}
      onCancel={(e) => {
        // Escape: rodzic decyduje o zamknięciu (odmontowaniu).
        e.preventDefault()
        onCloseRef.current()
      }}
      onClick={(e) => {
        // Kliknięcie w tło (poza zawartością okna).
        if (e.target === e.currentTarget) onCloseRef.current()
      }}
    >
      <div className={styles.body}>{children}</div>
    </dialog>
  )
}

export function CloseButton({ onClick }: { onClick: () => void }) {
  return (
    <button type="button" className={styles.close} aria-label="Zamknij" onClick={onClick}>
      <CloseIcon />
    </button>
  )
}

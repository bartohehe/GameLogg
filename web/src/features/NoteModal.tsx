import { useId, useState } from 'react'
import { CloseButton, Modal } from '../components/Modal'
import { useUpdateEntry, type Entry } from '../lib/games'
import styles from './NoteModal.module.css'

/** Szybka notatka do gry (z ekranu Odkryj). */
export function NoteModal({ entry, onClose }: { entry: Entry; onClose: () => void }) {
  const titleId = useId()
  const fieldId = useId()
  const [text, setText] = useState(entry.note)
  const update = useUpdateEntry()

  return (
    <Modal labelledBy={titleId} onClose={onClose} width={560}>
      <form
        className={styles.form}
        onSubmit={(e) => {
          e.preventDefault()
          update.mutate({ id: entry.id, patch: { note: text } }, { onSuccess: onClose })
        }}
      >
        <div className={styles.head}>
          <h1 id={titleId} className={styles.title}>
            Notatka: {entry.game.title}
          </h1>
          <CloseButton onClick={onClose} />
        </div>
        <label htmlFor={fieldId} className="sr-only">
          Treść notatki
        </label>
        <textarea
          id={fieldId}
          rows={6}
          maxLength={5000}
          value={text}
          placeholder="Gdzie jesteś, co zapamiętałeś, co dalej?"
          onChange={(e) => setText(e.target.value)}
          data-autofocus
        />
        <p role="alert" className={styles.error}>
          {update.error?.message ?? ''}
        </p>
        <div className={styles.actions}>
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            Anuluj
          </button>
          <button type="submit" className="btn btn-primary" aria-busy={update.isPending}>
            Zapisz notatkę
          </button>
        </div>
      </form>
    </Modal>
  )
}

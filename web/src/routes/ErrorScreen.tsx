interface Props {
  message: string
  onRetry?: () => void
}

/** Pełnoekranowy błąd, gdy aplikacja nie może wystartować (np. serwer nie odpowiada). */
export function ErrorScreen({ message, onRetry }: Props) {
  return (
    <main className="page" style={{ maxWidth: 720, paddingTop: 72 }}>
      <h1 className="page-title">Nie da się połączyć</h1>
      <p role="alert" className="page-lead" style={{ color: 'var(--warn)' }}>
        {message}
      </p>
      {onRetry && (
        <button type="button" className="btn btn-ghost" style={{ marginTop: 24 }} onClick={onRetry}>
          Spróbuj ponownie
        </button>
      )}
    </main>
  )
}

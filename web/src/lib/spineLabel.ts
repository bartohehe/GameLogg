/** aria-label grzbietu: „{tytuł}, {status}, ocena {score}” (ocena tylko, gdy jest). */
export function spineAriaLabel(title: string, statusLabel: string, score?: number | null): string {
  const parts = [title]
  if (statusLabel) parts.push(statusLabel)
  if (score != null) parts.push(`ocena ${score}`)
  return parts.join(', ')
}

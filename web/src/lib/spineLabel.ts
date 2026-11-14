import { STATUS, type Status } from './status'

/** aria-label grzbietu: „{tytuł}, {status}, ocena {score}” (ocena tylko, gdy jest). */
export function spineAriaLabel(title: string, status: Status, score?: number | null): string {
  const parts = [title, STATUS[status].label]
  if (score != null) parts.push(`ocena ${score}`)
  return parts.join(', ')
}

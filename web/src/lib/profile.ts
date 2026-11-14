import { countGames, plural } from './plural'

/** Fragment zdania: zwykły tekst albo wyróżniona liczba (score = kolor ocen). */
export type Part = string | { strong: string; score?: boolean }

interface Counts {
  total: number
  completed: number
  planned: number
  averageScore: number | null
}

/**
 * Statystyki w jednym zdaniu, z poprawną odmianą:
 * „Na półkach stoi 30 gier. 13 ukończonych, średnio na 88. W kolejce czeka 6 planowanych.”
 */
export function profileSentence(c: Counts): Part[] {
  if (c.total === 0) return ['Na półkach jeszcze nic nie stoi.']
  const parts: Part[] = [`Na półkach ${plural(c.total, 'stoi', 'stoją', 'stoi')} `, { strong: countGames(c.total) }, '.']
  if (c.completed > 0) {
    parts.push(' ', { strong: String(c.completed) }, ` ${plural(c.completed, 'ukończona', 'ukończone', 'ukończonych')}`)
    if (c.averageScore != null) parts.push(', średnio na ', { strong: String(c.averageScore), score: true })
    parts.push('.')
  }
  if (c.planned > 0) {
    parts.push(
      ` W kolejce ${plural(c.planned, 'czeka', 'czekają', 'czeka')} `,
      { strong: `${c.planned} ${plural(c.planned, 'planowana', 'planowane', 'planowanych')}` },
      '.',
    )
  }
  return parts
}

export function partsText(parts: Part[]): string {
  return parts.map((p) => (typeof p === 'string' ? p : p.strong)).join('')
}

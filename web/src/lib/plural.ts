/**
 * Polska odmiana liczebników:
 * one  — dokładnie 1 („1 gra”),
 * few  — końcówka 2–4, ale nie 12–14 („2 gry”, „23 gry”),
 * many — reszta, także 0, 5–21, 12–14, 25… („5 gier”, „12 gier”).
 */
export function plural(n: number, one: string, few: string, many: string): string {
  const abs = Math.abs(n)
  if (abs === 1) return one
  const last = abs % 10
  const lastTwo = abs % 100
  if (last >= 2 && last <= 4 && !(lastTwo >= 12 && lastTwo <= 14)) return few
  return many
}

/** „5 gier”, „2 gry”, „1 gra”. */
export const countGames = (n: number) => `${n} ${plural(n, 'gra', 'gry', 'gier')}`

/** Biernik: „grasz w 1 grę”, „w 2 gry”, „w 5 gier”. */
export const countGamesAcc = (n: number) => `${n} ${plural(n, 'grę', 'gry', 'gier')}`

/** „1 wynik”, „3 wyniki”, „12 wyników”. */
export const countResults = (n: number) => `${n} ${plural(n, 'wynik', 'wyniki', 'wyników')}`

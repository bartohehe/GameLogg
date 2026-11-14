// Formatowanie dat po polsku przez Intl (pl-PL).

/**
 * „marca 2022” — miesiąc w dopełniaczu z rokiem, do zdań typu „na półkach od …”.
 * Intl z samym { month, year } daje mianownik („marzec 2022”), a dopełniacz
 * zwraca tylko razem z dniem, więc bierzemy nazwę miesiąca z takiego formatu.
 */
export function monthYearGenitive(date: Date): string {
  const month = new Intl.DateTimeFormat('pl-PL', { day: 'numeric', month: 'long' })
    .formatToParts(date)
    .find((p) => p.type === 'month')?.value
  return `${month ?? ''} ${date.getFullYear()}`.trim()
}

/** „25 lutego 2022” */
export function longDate(date: Date): string {
  return new Intl.DateTimeFormat('pl-PL', { day: 'numeric', month: 'long', year: 'numeric' }).format(date)
}

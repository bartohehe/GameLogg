import { describe, expect, it } from 'vitest'
import { dayMonth, longDate, monthYearGenitive } from './format'

describe('monthYearGenitive', () => {
  it.each([
    [new Date(2022, 2, 3), 'marca 2022'],
    [new Date(2026, 10, 14), 'listopada 2026'],
    [new Date(2024, 0, 31), 'stycznia 2024'],
    [new Date(2023, 11, 1), 'grudnia 2023'],
  ])('%s → %s', (date, want) => {
    expect(monthYearGenitive(date)).toBe(want)
  })
})

describe('longDate', () => {
  it('formatuje pełną datę po polsku', () => {
    expect(longDate(new Date(2022, 1, 25))).toBe('25 lutego 2022')
  })
})

describe('dayMonth', () => {
  const now = new Date(2026, 10, 14)
  it('bez roku w bieżącym roku', () => {
    expect(dayMonth(new Date(2026, 8, 4), now)).toBe('4 września')
  })
  it('z rokiem dla innego roku', () => {
    expect(dayMonth(new Date(2024, 2, 3), now)).toBe('3 marca 2024')
  })
})

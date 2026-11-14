import { describe, expect, it } from 'vitest'
import { longDate, monthYearGenitive } from './format'

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

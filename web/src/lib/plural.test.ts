import { describe, expect, it } from 'vitest'
import { countGames, countGamesAcc, countResults, plural } from './plural'

describe('plural', () => {
  it.each([
    [0, 'gier'],
    [1, 'gra'],
    [2, 'gry'],
    [3, 'gry'],
    [4, 'gry'],
    [5, 'gier'],
    [11, 'gier'],
    [12, 'gier'],
    [13, 'gier'],
    [14, 'gier'],
    [21, 'gier'],
    [22, 'gry'],
    [23, 'gry'],
    [24, 'gry'],
    [25, 'gier'],
    [101, 'gier'],
    [102, 'gry'],
    [112, 'gier'],
    [114, 'gier'],
    [122, 'gry'],
  ])('%i → %s', (n, want) => {
    expect(plural(n, 'gra', 'gry', 'gier')).toBe(want)
  })
})

describe('gotowe odmiany', () => {
  it('biernik do „Grasz teraz w N …”', () => {
    expect(countGamesAcc(1)).toBe('1 grę')
    expect(countGamesAcc(4)).toBe('4 gry')
    expect(countGamesAcc(5)).toBe('5 gier')
    expect(countGamesAcc(13)).toBe('13 gier')
    expect(countGamesAcc(22)).toBe('22 gry')
  })

  it('mianownik i wyniki', () => {
    expect(countGames(1)).toBe('1 gra')
    expect(countGames(30)).toBe('30 gier')
    expect(countResults(1)).toBe('1 wynik')
    expect(countResults(3)).toBe('3 wyniki')
    expect(countResults(12)).toBe('12 wyników')
  })
})

import { describe, expect, it } from 'vitest'
// Te same przypadki czyta internal/ranking/ranking_test.go.
import cases from '../../../internal/ranking/testdata/cases.json'
import { CATEGORIES, insert, points, recompute, remove, score, WEIGHTS, type Category, type Place, type Rankings } from './ranking'

type Scores = Record<string, number | null>

interface RecomputeCase {
  name: string
  rankings: Rankings
  games: number[]
  want: Scores
  then?: {
    insert?: { category: Category; id: number; pos: number }
    remove?: number
    games: number[]
    rankings: Rankings
    want: Scores
  }
}

function asObject(r: Record<number, number | null>): Scores {
  return Object.fromEntries(Object.entries(r).map(([k, v]) => [k, v]))
}

describe('wagi', () => {
  it('sumują się do 100', () => {
    expect(CATEGORIES.reduce((s, c) => s + WEIGHTS[c], 0)).toBe(100)
  })
})

describe('points (wspólne przypadki z Go)', () => {
  it.each(cases.points)('p=$p n=$n → $want', ({ p, n, want }) => {
    expect(points(p, n)).toBeCloseTo(want, 9)
  })
})

describe('score (wspólne przypadki z Go)', () => {
  it.each(cases.scores)('$name', ({ places, want }) => {
    expect(score(places as Partial<Record<Category, Place>>)).toBe(want)
  })
})

describe('recompute (wspólne przypadki z Go)', () => {
  it.each(cases.recompute as RecomputeCase[])('$name', (tc) => {
    expect(asObject(recompute(tc.rankings, tc.games))).toEqual(tc.want)
    if (!tc.then) return

    const next: Rankings = {}
    for (const c of CATEGORIES) if (tc.rankings[c]) next[c] = [...tc.rankings[c]]
    if (tc.then.insert) {
      const { category, id, pos } = tc.then.insert
      next[category] = insert(next[category] ?? [], id, pos)
    }
    if (tc.then.remove != null) {
      for (const c of CATEGORIES) if (next[c]) next[c] = remove(next[c], tc.then.remove)
    }
    for (const c of CATEGORIES) expect(next[c] ?? undefined).toEqual(tc.then.rankings[c] ?? undefined)
    expect(asObject(recompute(next, tc.then.games))).toEqual(tc.then.want)
  })
})

describe('insert', () => {
  it('nie zmienia listy wejściowej i przycina pozycję', () => {
    const list = [1, 2]
    expect(insert(list, 3, 99)).toEqual([1, 2, 3])
    expect(insert(list, 3, -1)).toEqual([3, 1, 2])
    expect(insert([1, 2, 3], 1, 2)).toEqual([2, 3, 1])
    expect(list).toEqual([1, 2])
  })
})

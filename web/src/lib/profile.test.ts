import { describe, expect, it } from 'vitest'
import { partsText, profileSentence } from './profile'

const text = (total: number, completed: number, planned: number, averageScore: number | null) =>
  partsText(profileSentence({ total, completed, planned, averageScore }))

describe('profileSentence', () => {
  it('pusta biblioteka', () => {
    expect(text(0, 0, 0, null)).toBe('Na półkach jeszcze nic nie stoi.')
  })

  it('jak w mockupie', () => {
    expect(text(30, 13, 6, 88)).toBe('Na półkach stoi 30 gier. 13 ukończonych, średnio na 88. W kolejce czeka 6 planowanych.')
  })

  it('odmienia liczebniki', () => {
    expect(text(1, 1, 0, 100)).toBe('Na półkach stoi 1 gra. 1 ukończona, średnio na 100.')
    expect(text(2, 0, 2, null)).toBe('Na półkach stoją 2 gry. W kolejce czekają 2 planowane.')
    expect(text(23, 22, 1, null)).toBe('Na półkach stoją 23 gry. 22 ukończone. W kolejce czeka 1 planowana.')
  })

  it('wyróżnia średnią kolorem ocen', () => {
    const parts = profileSentence({ total: 3, completed: 1, planned: 0, averageScore: 80 })
    expect(parts).toContainEqual({ strong: '80', score: true })
  })
})

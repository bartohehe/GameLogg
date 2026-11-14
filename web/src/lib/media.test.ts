import { describe, expect, it } from 'vitest'
import { countEpisodes, countTitles, formatRuntime, lengthText, mediaSpineSize } from './media'

describe('media', () => {
  it('formatuje czas trwania', () => {
    expect(formatRuntime(166)).toBe('2 h 46 min')
    expect(formatRuntime(58)).toBe('58 min')
    expect(formatRuntime(120)).toBe('2 h')
  })

  it('odmienia odcinki i tytuły', () => {
    expect(countEpisodes(1)).toBe('1 odcinek')
    expect(countEpisodes(6)).toBe('6 odcinków')
    expect(countEpisodes(22)).toBe('22 odcinki')
    expect(countTitles(18)).toBe('18 tytułów')
    expect(countTitles(3)).toBe('3 tytuły')
  })

  it('długość: film w minutach, serial w odcinkach', () => {
    expect(lengthText({ type: 'film', runtimeMin: 166, episodes: null })).toBe('2 h 46 min')
    expect(lengthText({ type: 'serial', runtimeMin: null, episodes: 6 })).toBe('6 odcinków')
    expect(lengthText({ type: 'anime', runtimeMin: 58, episodes: null })).toBe('58 min')
    expect(lengthText({ type: 'serial', runtimeMin: null, episodes: null })).toBe('')
  })

  it('szerokość grzbietu zależy od rodzaju', () => {
    expect(mediaSpineSize('serial').width).toBe(52)
    expect(mediaSpineSize('anime').width).toBe(40)
    expect(mediaSpineSize('film').width).toBe(32)
  })
})

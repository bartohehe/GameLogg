import { createContext, useContext } from 'react'
import type { SpineColors } from '../../lib/games'

/** Gra, którą oceniamy (wpis w bibliotece ze statusem Ukończone). */
export interface RankingTarget {
  entryId: number
  title: string
  developer: string
  year: number | null
  coverUrl: string | null
  spine: SpineColors
}

export const RankingContext = createContext<((t: RankingTarget) => void) | null>(null)

/** Otwiera okno oceny w rankingu. Działa w drzewie pod RankingProvider (AppLayout). */
export function useOpenRanking() {
  const open = useContext(RankingContext)
  if (!open) throw new Error('useOpenRanking poza RankingProvider')
  return open
}

/** Wygodny skrót: cel oceny z wpisu biblioteki. */
export function targetFromEntry(e: {
  id: number
  game: { title: string; developer: string; year: number | null; coverUrl: string | null; spine: SpineColors }
}): RankingTarget {
  return {
    entryId: e.id,
    title: e.game.title,
    developer: e.game.developer,
    year: e.game.year,
    coverUrl: e.game.coverUrl,
    spine: e.game.spine,
  }
}

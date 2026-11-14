import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from './api'
import type { Category } from './ranking'
import type { Status } from './status'

export interface SpineColors {
  color: string
  accent: string
  ink: string
}

export interface Game {
  igdbId: number
  title: string
  coverUrl: string | null
  releaseDate: string | null
  year: number | null
  developer: string
  genres: string[]
  platforms: string[]
  igdbRating: number | null
  /** null dla gier, których nikt jeszcze nie postawił na półce. */
  spine: SpineColors | null
}

export interface EntryRef {
  id: number
  status: Status
}

export interface SearchResult extends Game {
  entry: EntryRef | null
}

export interface Entry {
  id: number
  status: Status
  platform: string
  score: number | null
  note: string
  completedCount: number
  finishedAt: string | null
  addedAt: string
  updatedAt: string
  game: Game & { spine: SpineColors }
}

export interface GameDetails extends Game {
  summary: string
  summaryTranslated: string | null
  entry: Entry | null
}

export type LibrarySort = 'score' | 'title' | 'added'

export interface EntryPatch {
  status?: Status
  platform?: string
  note?: string
}

export const gamesApi = {
  search: (q: string) =>
    apiFetch<{ results: SearchResult[] }>(`/games/search?q=${encodeURIComponent(q)}`).then((r) => r.results),
  details: (igdbId: number) => apiFetch<GameDetails>(`/games/${igdbId}`),
  library: (sort: LibrarySort) => apiFetch<{ items: Entry[] }>(`/library?sort=${sort}`).then((r) => r.items),
  add: (igdbId: number, status: Status, platform: string) =>
    apiFetch<Entry>('/library', 'POST', { igdbId, status, platform }),
  update: (id: number, patch: EntryPatch) => apiFetch<Entry>(`/library/${id}`, 'PATCH', patch),
  remove: (id: number) => apiFetch<void>(`/library/${id}`, 'DELETE'),
}

export const libraryKey = ['library'] as const

export function useLibrary(sort: LibrarySort = 'score') {
  return useQuery({ queryKey: [...libraryKey, sort], queryFn: () => gamesApi.library(sort) })
}

/** Po każdej zmianie półek odświeżamy wszystko, co pokazuje stan biblioteki. */
export function useInvalidateLibrary() {
  const qc = useQueryClient()
  return () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: libraryKey }),
      qc.invalidateQueries({ queryKey: ['search'] }),
      qc.invalidateQueries({ queryKey: ['game'] }),
      qc.invalidateQueries({ queryKey: ['ranking'] }),
      qc.invalidateQueries({ queryKey: ['entryRankings'] }),
      qc.invalidateQueries({ queryKey: ['stats'] }),
      qc.invalidateQueries({ queryKey: ['popular'] }),
    ])
}

export function useAddGame() {
  const invalidate = useInvalidateLibrary()
  return useMutation({
    mutationFn: (v: { igdbId: number; status: Status; platform: string }) => gamesApi.add(v.igdbId, v.status, v.platform),
    onSuccess: invalidate,
  })
}

export function useUpdateEntry() {
  const invalidate = useInvalidateLibrary()
  return useMutation({
    mutationFn: (v: { id: number; patch: EntryPatch }) => gamesApi.update(v.id, v.patch),
    onSuccess: invalidate,
  })
}

export function useRemoveEntry() {
  const invalidate = useInvalidateLibrary()
  return useMutation({ mutationFn: (id: number) => gamesApi.remove(id), onSuccess: invalidate })
}

// --- Rankingi ---

export interface RankedItem {
  position: number
  entry: Entry
}

export interface CategoryPlace {
  category: Category
  weight: number
  /** null = gra pominięta w tej kategorii. */
  position: number | null
  n: number
  points: number | null
}

export interface EntryRankings {
  score: number | null
  categories: CategoryPlace[]
}

export const rankingsApi = {
  list: (c: Category) =>
    apiFetch<{ items: RankedItem[] }>(`/rankings/${c}`).then((r) => r.items),
  put: (c: Category, userGameIds: number[]) =>
    apiFetch<{ scores: Array<{ id: number; score: number | null }> }>(`/rankings/${c}`, 'PUT', { userGameIds }),
  forEntry: (entryId: number) => apiFetch<EntryRankings>(`/library/${entryId}/rankings`),
}

export const rankingKey = (c: Category) => ['ranking', c] as const
export const entryRankingsKey = (id: number) => ['entryRankings', id] as const

export function useEntryRankings(entryId: number | null, enabled = true) {
  return useQuery({
    queryKey: entryRankingsKey(entryId ?? 0),
    queryFn: () => rankingsApi.forEntry(entryId!),
    enabled: enabled && entryId != null,
  })
}

// --- Odkryj ---

export interface Recommendation {
  game: Game
  because: { igdbId: number; title: string; score: number }
}

export interface YearStats {
  year: number
  completed: number
  abandoned: number
  averageScore: number | null
  top: Entry[]
}

export interface Stats {
  counts: Record<Status, number>
  total: number
  averageScore: number | null
  top: Entry[]
  year: YearStats
}

export const discoverApi = {
  popular: () => apiFetch<{ results: SearchResult[] }>('/games/popular').then((r) => r.results),
  recommendations: () => apiFetch<{ items: Recommendation[]; basedOn: number }>('/games/recommendations'),
  stats: (year: number) => apiFetch<Stats>(`/stats?year=${year}`),
}

export function useStats(year = new Date().getFullYear()) {
  return useQuery({ queryKey: ['stats', year], queryFn: () => discoverApi.stats(year) })
}

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from './api'
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
function useInvalidateLibrary() {
  const qc = useQueryClient()
  return () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: libraryKey }),
      qc.invalidateQueries({ queryKey: ['search'] }),
      qc.invalidateQueries({ queryKey: ['game'] }),
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

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from './api'
import type { SpineColors } from './games'
import type { SpineSize } from './platforms'
import { plural } from './plural'

export type MediaType = 'film' | 'serial' | 'anime'
export type MediaStatus = 'backlog' | 'watching' | 'watched'
export type MediaSource = 'tmdb' | 'jikan'

export const TYPE_LABEL: Record<MediaType, string> = { film: 'Film', serial: 'Serial', anime: 'Anime' }

export const MEDIA_STATUS: Record<MediaStatus, { label: string; color: string }> = {
  backlog: { label: 'Do obejrzenia', color: 'var(--st-planned)' },
  watching: { label: 'Oglądam', color: 'var(--st-in-progress)' },
  watched: { label: 'Obejrzane', color: 'var(--st-completed)' },
}

export const MEDIA_STATUS_ORDER: MediaStatus[] = ['backlog', 'watching', 'watched']

/** Grzbiety mediów: seriale 52 px, anime 40 px, filmy 32 px szerokości. */
export function mediaSpineSize(t: MediaType): SpineSize {
  switch (t) {
    case 'serial':
      return { width: 52, height: 190 }
    case 'anime':
      return { width: 40, height: 178 }
    default:
      return { width: 32, height: 184 }
  }
}

export const SOURCE_LABEL: Record<MediaSource, string> = { tmdb: 'TMDB', jikan: 'MAL' }

export interface MediaRef {
  id: number
  status: MediaStatus
}

export interface MediaResult {
  source: MediaSource
  externalId: string
  type: MediaType
  title: string
  year: number | null
  creator: string
  genres: string[]
  coverUrl: string | null
  score: number | null
  episodes: number | null
  runtimeMin: number | null
  entry: MediaRef | null
}

export interface MediaItem {
  id: number
  source: MediaSource
  externalId: string
  type: MediaType
  title: string
  year: number | null
  creator: string
  genres: string[]
  coverUrl: string | null
  runtimeMin: number | null
  episodes: number | null
  watchedEpisodes: number | null
  status: MediaStatus
  score: number | null
  note: string
  spine: SpineColors
  addedAt: string
  updatedAt: string
}

export interface MediaSearch {
  results: MediaResult[]
  tmdbConfigured: boolean
  errors: Array<{ source: MediaSource; code: string; message: string }>
}

export interface MediaPatch {
  status?: MediaStatus
  /** null czyści ocenę. */
  score?: number | null
  watchedEpisodes?: number
  note?: string
}

export const mediaApi = {
  list: () => apiFetch<{ items: MediaItem[] }>('/media').then((r) => r.items),
  search: (q: string) => apiFetch<MediaSearch>(`/media/search?q=${encodeURIComponent(q)}`),
  add: (r: Pick<MediaResult, 'source' | 'externalId' | 'type'>, status: MediaStatus = 'backlog') =>
    apiFetch<MediaItem>('/media', 'POST', { source: r.source, externalId: r.externalId, type: r.type, status }),
  update: (id: number, patch: MediaPatch) => apiFetch<MediaItem>(`/media/${id}`, 'PATCH', patch),
  episode: (id: number) => apiFetch<MediaItem>(`/media/${id}/episode`, 'POST'),
  remove: (id: number) => apiFetch<void>(`/media/${id}`, 'DELETE'),
}

export const mediaKey = ['media'] as const

export function useMediaList(enabled = true) {
  return useQuery({ queryKey: mediaKey, queryFn: mediaApi.list, enabled })
}

function useInvalidateMedia() {
  const qc = useQueryClient()
  return () => Promise.all([qc.invalidateQueries({ queryKey: mediaKey }), qc.invalidateQueries({ queryKey: ['mediaSearch'] })])
}

export function useAddMedia() {
  const invalidate = useInvalidateMedia()
  return useMutation({
    mutationFn: (v: { result: MediaResult; status?: MediaStatus }) => mediaApi.add(v.result, v.status),
    onSuccess: invalidate,
  })
}

export function useUpdateMedia() {
  const invalidate = useInvalidateMedia()
  return useMutation({ mutationFn: (v: { id: number; patch: MediaPatch }) => mediaApi.update(v.id, v.patch), onSuccess: invalidate })
}

export function useEpisode() {
  const invalidate = useInvalidateMedia()
  return useMutation({ mutationFn: (id: number) => mediaApi.episode(id), onSuccess: invalidate })
}

export function useRemoveMedia() {
  const invalidate = useInvalidateMedia()
  return useMutation({ mutationFn: (id: number) => mediaApi.remove(id), onSuccess: invalidate })
}

/** „2 h 46 min”, „58 min”. */
export function formatRuntime(min: number): string {
  const h = Math.floor(min / 60)
  const m = min % 60
  if (h === 0) return `${m} min`
  return m === 0 ? `${h} h` : `${h} h ${m} min`
}

export const countEpisodes = (n: number) => `${n} ${plural(n, 'odcinek', 'odcinki', 'odcinków')}`
export const countTitles = (n: number) => `${n} ${plural(n, 'tytuł', 'tytuły', 'tytułów')}`

/** Długość tytułu: czas filmu albo liczba odcinków. */
export function lengthText(m: { type: MediaType; runtimeMin: number | null; episodes: number | null }): string {
  if (m.type === 'film' || (m.runtimeMin && !m.episodes)) return m.runtimeMin ? formatRuntime(m.runtimeMin) : ''
  return m.episodes ? countEpisodes(m.episodes) : ''
}

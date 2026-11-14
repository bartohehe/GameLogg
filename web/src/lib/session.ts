import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { api, isUnauthorized } from './api'

export const configKey = ['config'] as const
export const meKey = ['me'] as const

/** Konfiguracja serwera. Ładowana raz przy starcie (Suspense), potem z cache. */
export function useConfig() {
  return useSuspenseQuery({ queryKey: configKey, queryFn: api.config, staleTime: Infinity }).data
}

/** Zalogowany użytkownik. 401 nie jest ponawiane: oznacza brak sesji. */
export function useMe() {
  return useQuery({
    queryKey: meKey,
    queryFn: api.me,
    staleTime: 5 * 60_000,
    retry: (count, err) => !isUnauthorized(err) && count < 2,
  })
}

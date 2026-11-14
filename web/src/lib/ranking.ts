// Ta sama logika co internal/ranking (Go). Oba testy czytają wspólne przypadki
// z internal/ranking/testdata/cases.json, więc podgląd na żywo w oknie oceny
// liczy dokładnie to, co potem zapisze backend.

export type Category = 'gameplay' | 'story' | 'graphics' | 'audio' | 'replayability' | 'difficulty'

/** Kategorie w stałej kolejności (ta sama w Go). */
export const CATEGORIES: Category[] = ['gameplay', 'story', 'graphics', 'audio', 'replayability', 'difficulty']

export const WEIGHTS: Record<Category, number> = {
  gameplay: 35,
  story: 25,
  graphics: 15,
  audio: 10,
  replayability: 10,
  difficulty: 5,
}

export const CATEGORY_LABEL: Record<Category, string> = {
  gameplay: 'Gameplay',
  story: 'Fabuła',
  graphics: 'Grafika',
  audio: 'Audio',
  replayability: 'Regrywalność',
  difficulty: 'Trudność',
}

export interface Place {
  /** 1 = najlepsze miejsce. */
  pos: number
  /** Liczba gier w rankingu kategorii. */
  n: number
}

/** Punkty za miejsce p w rankingu o n grach: 100 − (p−1)/(n−1)·40; przy n = 1 zawsze 100. */
export function points(p: number, n: number): number {
  if (n <= 1) return 100
  return 100 - ((p - 1) / (n - 1)) * 40
}

/** Średnia ważona z kategorii z miejscem, zaokrąglona. Bez kategorii: null. */
export function score(places: Partial<Record<Category, Place>>): number | null {
  let sum = 0
  let weights = 0
  for (const c of CATEGORIES) {
    const p = places[c]
    if (!p) continue
    sum += points(p.pos, p.n) * WEIGHTS[c]
    weights += WEIGHTS[c]
  }
  if (weights === 0) return null
  return Math.round(sum / weights)
}

export type Rankings = Partial<Record<Category, number[]>>

export function placesOf(r: Rankings, id: number): Partial<Record<Category, Place>> {
  const out: Partial<Record<Category, Place>> = {}
  for (const c of CATEGORIES) {
    const list = r[c] ?? []
    const i = list.indexOf(id)
    if (i >= 0) out[c] = { pos: i + 1, n: list.length }
  }
  return out
}

/** Oceny wszystkich podanych gier; gry spoza rankingów dostają null. */
export function recompute(r: Rankings, games: number[]): Record<number, number | null> {
  const out: Record<number, number | null> = {}
  for (const id of games) out[id] = score(placesOf(r, id))
  return out
}

export function remove(list: number[], id: number): number[] {
  return list.filter((v) => v !== id)
}

/** Nowa kolejność z grą id na pozycji pos (0-based); wcześniejsze miejsce gry znika. */
export function insert(list: number[], id: number, pos: number): number[] {
  const out = remove(list, id)
  const at = Math.max(0, Math.min(pos, out.length))
  out.splice(at, 0, id)
  return out
}

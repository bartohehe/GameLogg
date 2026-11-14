export type Status = 'planned' | 'in_progress' | 'completed' | 'on_hold' | 'wishlist' | 'abandoned'

interface StatusInfo {
  label: string
  /** Kolor statusu jako zmienna CSS z tokens.css. */
  color: string
  /** Krótki opis półki w oknie dodawania. */
  hint: string
}

export const STATUS: Record<Status, StatusInfo> = {
  in_progress: { label: 'W trakcie', color: 'var(--st-in-progress)', hint: 'Gram teraz' },
  planned: { label: 'Planowane', color: 'var(--st-planned)', hint: 'Mam zamiar zagrać' },
  completed: { label: 'Ukończone', color: 'var(--st-completed)', hint: 'Skończyłem, od razu ocenię' },
  on_hold: { label: 'Wstrzymane', color: 'var(--st-on-hold)', hint: 'Odłożyłem na później' },
  wishlist: { label: 'Lista życzeń', color: 'var(--st-wishlist)', hint: 'Chcę kupić' },
  abandoned: { label: 'Porzucone', color: 'var(--st-abandoned)', hint: 'Nie dokończę' },
}

/** Kolejność półek w bibliotece. */
export const SHELF_ORDER: Status[] = ['in_progress', 'planned', 'completed', 'on_hold', 'wishlist', 'abandoned']

/** Kolejność w siatce 3×2 okna dodawania (jak w mockupie). */
export const ADD_ORDER: Status[] = ['planned', 'in_progress', 'completed', 'wishlist', 'on_hold', 'abandoned']

export function isStatus(v: string | null | undefined): v is Status {
  return v != null && v in STATUS
}

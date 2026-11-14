// Cienki klient API. Błędy backendu mają zawsze postać
// {"error": {"code": "...", "message": "..."}}, a message jest gotowym tekstem dla użytkownika.

export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

const FALLBACK_MESSAGE = 'Serwer GameLogg nie odpowiada. Sprawdź, czy kontener app działa.'

function isErrorBody(v: unknown): v is { error: { code: string; message: string } } {
  if (typeof v !== 'object' || v === null || !('error' in v)) return false
  const e = (v as { error: unknown }).error
  return (
    typeof e === 'object' &&
    e !== null &&
    typeof (e as { code?: unknown }).code === 'string' &&
    typeof (e as { message?: unknown }).message === 'string'
  )
}

/** Zamienia nieudaną odpowiedź HTTP na ApiError z czytelnym komunikatem. */
export async function toApiError(res: Response): Promise<ApiError> {
  let body: unknown = null
  try {
    body = await res.json()
  } catch {
    // Odpowiedź bez JSON-a (np. 502 z reverse proxy).
  }
  if (isErrorBody(body)) {
    return new ApiError(res.status, body.error.code, body.error.message)
  }
  return new ApiError(res.status, 'unexpected_response', FALLBACK_MESSAGE)
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(`/api${path}`, {
      credentials: 'same-origin',
      ...init,
      headers: {
        Accept: 'application/json',
        ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
        ...init?.headers,
      },
    })
  } catch {
    throw new ApiError(0, 'network_error', FALLBACK_MESSAGE)
  }
  if (!res.ok) throw await toApiError(res)
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export interface Health {
  status: 'ok'
  version: string
}

export const getHealth = () => apiFetch<Health>('/health')

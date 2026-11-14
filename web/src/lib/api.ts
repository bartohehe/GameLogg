// Cienki klient API. Błędy backendu mają zawsze postać
// {"error": {"code": "...", "message": "...", "field"?: "..."}}, a message jest
// gotowym tekstem dla użytkownika.

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  /** Pole formularza, którego dotyczy błąd (jeśli backend je wskazał). */
  readonly field?: string

  constructor(status: number, code: string, message: string, field?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.field = field
  }
}

const FALLBACK_MESSAGE = 'Serwer GameLogg nie odpowiada. Sprawdź, czy kontener app działa.'

interface ErrorBody {
  error: { code: string; message: string; field?: string }
}

function isErrorBody(v: unknown): v is ErrorBody {
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
    const { code, message, field } = body.error
    return new ApiError(res.status, code, message, typeof field === 'string' ? field : undefined)
  }
  return new ApiError(res.status, 'unexpected_response', FALLBACK_MESSAGE)
}

type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

export async function apiFetch<T>(path: string, method: Method = 'GET', body?: unknown): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  // Backend wymaga application/json przy każdej zmianie stanu (ochrona przed CSRF).
  if (method !== 'GET') headers['Content-Type'] = 'application/json'

  let res: Response
  try {
    res = await fetch(`/api${path}`, {
      method,
      headers,
      credentials: 'same-origin',
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    throw new ApiError(0, 'network_error', FALLBACK_MESSAGE)
  }
  if (!res.ok) throw await toApiError(res)
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export function isUnauthorized(err: unknown): boolean {
  return err instanceof ApiError && err.status === 401
}

// --- Typy i wywołania ---

export interface ServerConfig {
  authEnabled: boolean
  mediaEnabled: boolean
  translationEnabled: boolean
  igdbConfigured: boolean
  tmdbConfigured: boolean
  version: string
}

export interface User {
  id: number
  username: string
  email: string
  createdAt: string
}

export const api = {
  config: () => apiFetch<ServerConfig>('/config'),
  me: () => apiFetch<User>('/me'),
  login: (email: string, password: string) => apiFetch<User>('/auth/login', 'POST', { email, password }),
  register: (username: string, email: string, password: string) =>
    apiFetch<User>('/auth/register', 'POST', { username, email, password }),
  logout: () => apiFetch<void>('/auth/logout', 'POST'),
}

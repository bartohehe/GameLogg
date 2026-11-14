import { describe, expect, it } from 'vitest'
import { ApiError, toApiError } from './api'

describe('toApiError', () => {
  it('czyta kod i komunikat z formatu błędów backendu', async () => {
    const res = new Response(
      JSON.stringify({ error: { code: 'igdb_not_configured', message: 'Brak kluczy IGDB.' } }),
      { status: 503 },
    )
    const err = await toApiError(res)
    expect(err).toBeInstanceOf(ApiError)
    expect(err.status).toBe(503)
    expect(err.code).toBe('igdb_not_configured')
    expect(err.message).toBe('Brak kluczy IGDB.')
  })

  it('daje ogólny komunikat, gdy odpowiedź nie jest JSON-em', async () => {
    const err = await toApiError(new Response('Bad Gateway', { status: 502 }))
    expect(err.status).toBe(502)
    expect(err.code).toBe('unexpected_response')
    expect(err.message).toMatch(/kontener app/)
  })

  it('daje ogólny komunikat, gdy JSON ma inny kształt', async () => {
    const err = await toApiError(new Response(JSON.stringify({ msg: 'x' }), { status: 500 }))
    expect(err.code).toBe('unexpected_response')
  })
})

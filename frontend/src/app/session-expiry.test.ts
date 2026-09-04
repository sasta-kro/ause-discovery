// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest'
import { client } from '../api/generated/client.gen'
import { installSessionExpiryNotification, sessionExpiryListenerCount } from './session-expiry'

function unauthorizedProblemBody(): string {
  return JSON.stringify({ code: 'unauthorized', status: 401, title: 'Unauthorized', type: 'about:blank', request_id: 'test' })
}

describe('session expiry notification', () => {
  it('reports authenticated 401 responses and installs exactly one interceptor', async () => {
    const firstListener = vi.fn()
    const activeListener = vi.fn()
    installSessionExpiryNotification(firstListener)
    expect(sessionExpiryListenerCount()).toBe(1)

    installSessionExpiryNotification(activeListener)
    expect(sessionExpiryListenerCount()).toBe(1)

    const fetchMock = vi.fn(async () => new Response(unauthorizedProblemBody(), { status: 401, headers: { 'Content-Type': 'application/json' } }))
    client.setConfig({ baseUrl: 'http://session-expiry.test', fetch: fetchMock as unknown as typeof fetch, throwOnError: true })

    await expect(client.get({ url: '/probe' })).rejects.toBeTruthy()
    expect(firstListener).not.toHaveBeenCalled()
    expect(activeListener).toHaveBeenCalledTimes(1)

    await expect(client.get({ url: '/probe' })).rejects.toBeTruthy()
    expect(activeListener).toHaveBeenCalledTimes(2)
    expect(sessionExpiryListenerCount()).toBe(1)
  })

  it('ignores failures other than 401 responses', async () => {
    const listener = vi.fn()
    installSessionExpiryNotification(listener)
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ code: 'internal_error', status: 500, title: 'Error', type: 'about:blank', request_id: 'test' }), { status: 500, headers: { 'Content-Type': 'application/json' } }))
    client.setConfig({ baseUrl: 'http://session-expiry.test', fetch: fetchMock as unknown as typeof fetch, throwOnError: true })

    await expect(client.get({ url: '/probe' })).rejects.toBeTruthy()
    expect(listener).not.toHaveBeenCalled()
  })
})

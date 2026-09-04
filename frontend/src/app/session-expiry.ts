import { client } from '../api/generated/client.gen'

type SessionExpiryListener = () => void

let listener: SessionExpiryListener | null = null
let installed = false

/**
 * Registers one generated-client error interceptor that reports authenticated
 * 401 responses so cached administrator session state can be cleared. Only
 * status codes are inspected; response bodies and request details are never
 * read or logged.
 */
export function installSessionExpiryNotification(onSessionExpired: SessionExpiryListener): void {
  listener = onSessionExpired
  if (installed) return
  installed = true
  client.interceptors.error.use((error, response) => {
    if (response?.status === 401) listener?.()
    return error
  })
}

export function sessionExpiryListenerCount(): number {
  return installed ? 1 : 0
}

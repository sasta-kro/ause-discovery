import { normalizePublicBasePath } from './base-path-core'

export { normalizePublicBasePath }

// The fallback mirrors vite.config.ts so dev and build agree when the
// environment variable is absent: the public subpath everywhere except tests.
export const publicBasePath = normalizePublicBasePath(
  import.meta.env.VITE_PUBLIC_BASE_PATH ?? (import.meta.env.MODE === 'test' ? '/' : '/ause-discovery/'),
)

export function buildApiUrl(path: string): string {
  const normalizedPath = path.replace(/^\/+/, '')
  const apiPath = import.meta.env.VITE_API_PATH?.replace(/^\/+|\/+$/g, '') || 'api/v1'

  return `${publicBasePath}${apiPath}/${normalizedPath}`
}

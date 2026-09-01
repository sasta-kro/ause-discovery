import { normalizePublicBasePath } from './base-path-core'

export { normalizePublicBasePath }

export const publicBasePath = normalizePublicBasePath(import.meta.env.VITE_PUBLIC_BASE_PATH)

export function buildApiUrl(path: string): string {
  const normalizedPath = path.replace(/^\/+/, '')
  const apiPath = import.meta.env.VITE_API_PATH?.replace(/^\/+|\/+$/g, '') || 'api/v1'

  return `${publicBasePath}${apiPath}/${normalizedPath}`
}

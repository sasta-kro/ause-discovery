import { client } from '../api/generated/client.gen'
import { publicBasePath } from './public-base-path'

export function buildApiBaseUrl(
  basePath = publicBasePath,
  apiPath = import.meta.env.VITE_API_PATH ?? 'api/v1',
): string {
  const normalizedApiPath = apiPath.replace(/^\/+|\/+$/g, '')
  return `${basePath}${normalizedApiPath}`
}

export function routerBasenameFor(basePath: string): string {
  return basePath === '/' ? '/' : basePath.slice(0, -1)
}

export const routerBasename = routerBasenameFor(publicBasePath)

export function configureApiClient(): void {
  client.setConfig({
    baseUrl: buildApiBaseUrl(),
    credentials: 'include',
  })
}

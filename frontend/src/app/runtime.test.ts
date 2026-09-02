import { describe, expect, it } from 'vitest'
import { buildApiBaseUrl, routerBasenameFor } from './runtime'

describe('base-path-safe frontend runtime', () => {
  it.each([['/', 'api/v1', '/api/v1', '/'], ['/ause-discovery/', 'api/v1', '/ause-discovery/api/v1', '/ause-discovery'], ['/records/', '/api/v1/', '/records/api/v1', '/records']])('builds API and router paths for %s', (basePath, apiPath, expectedApiPath, expectedRouterPath) => {
    expect(buildApiBaseUrl(basePath, apiPath)).toBe(expectedApiPath)
    expect(routerBasenameFor(basePath)).toBe(expectedRouterPath)
  })
})

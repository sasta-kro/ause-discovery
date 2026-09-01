const duplicateSlashPattern = /\/{2,}/g

export function normalizePublicBasePath(value: string | undefined): string {
  const trimmedValue = value?.trim() ?? ''

  if (trimmedValue.includes('?') || trimmedValue.includes('#')) {
    throw new Error('VITE_PUBLIC_BASE_PATH cannot contain a query string or fragment')
  }

  const pathWithoutLeadingSlash = trimmedValue.replace(/^\/+/, '')
  const normalizedSegments = pathWithoutLeadingSlash
    .replace(duplicateSlashPattern, '/')
    .split('/')
    .filter(Boolean)

  return normalizedSegments.length === 0 ? '/' : `/${normalizedSegments.join('/')}/`
}

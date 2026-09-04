import type { Problem } from '../api/generated/types.gen'

export function isProblem(value: unknown): value is Problem {
  return typeof value === 'object' && value !== null && 'code' in value && 'status' in value
}

export function isNotFoundFailure(error: unknown): boolean {
  return isProblem(error) && error.code === 'not_found'
}

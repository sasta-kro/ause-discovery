import { z } from 'zod'
import type { SearchProjectsData } from '../../api/generated/types.gen'

const arrayKeys = ['program_key', 'course_key', 'person_id', 'advisor_id', 'category_key', 'platform_key', 'domain_key', 'technology_key', 'artifact_type'] as const
const booleanKeys = ['has_artifacts', 'has_report', 'has_slides', 'has_source_code', 'has_dataset'] as const

export type SearchState = NonNullable<SearchProjectsData['query']>

const sortSchema = z.enum(['relevance', 'newest', 'oldest', 'title'])
const semesterSchema = z.enum(['first', 'second', 'summer'])

function values(parameters: URLSearchParams, key: string): string[] | undefined {
  const selected = parameters.getAll(key).filter(Boolean)
  return selected.length > 0 ? selected : undefined
}

function optionalBoolean(parameters: URLSearchParams, key: string): boolean | undefined {
  const value = parameters.get(key)
  return value === 'true' ? true : value === 'false' ? false : undefined
}

export function parseSearchState(parameters: URLSearchParams): SearchState {
  const academicYear = Number(parameters.get('academic_year'))
  const limit = Number(parameters.get('limit'))
  const semester = semesterSchema.safeParse(parameters.get('semester'))
  const sort = sortSchema.safeParse(parameters.get('sort'))
  const state: SearchState = {
    q: parameters.get('q') || undefined,
    academic_year: Number.isInteger(academicYear) && academicYear > 1900 && academicYear < 3000 ? academicYear : undefined,
    semester: semester.success ? semester.data : undefined,
    student_id: /^\d{7}$/.test(parameters.get('student_id') ?? '') ? parameters.get('student_id') ?? undefined : undefined,
    cursor: parameters.get('cursor') || undefined,
    limit: Number.isInteger(limit) && limit >= 1 && limit <= 100 ? limit : 20,
    // An omitted or invalid sort stays undefined so the contextual default
    // (newest for an empty search, relevance for text) survives in state.
    sort: sort.success ? sort.data : undefined,
  }
  for (const key of arrayKeys) Object.assign(state, { [key]: values(parameters, key) })
  for (const key of booleanKeys) Object.assign(state, { [key]: optionalBoolean(parameters, key) })
  return state
}

export function serializeSearchState(state: SearchState): URLSearchParams {
  const parameters = new URLSearchParams()
  for (const [key, value] of Object.entries(state)) {
    if (key === 'major_key' || key === 'topic_key') continue
    // Every explicit valid sort, including relevance, serializes; only the
    // implicit undefined state stays omitted.
    if (value === undefined || value === '' || (key === 'limit' && value === 20)) continue
    if (Array.isArray(value)) value.forEach((item) => parameters.append(key, item))
    else parameters.set(key, String(value))
  }
  return parameters
}

export function resetSearchCursor(state: SearchState): SearchState {
  return { ...state, cursor: undefined }
}

// The effective selection for display: an explicit sort is authoritative;
// an implicit state defaults to newest for an empty or whitespace-only
// query and relevance once text exists.
export function displayedSort(state: SearchState): NonNullable<SearchState['sort']> {
  if (state.sort) return state.sort
  return (state.q ?? '').trim() === '' ? 'newest' : 'relevance'
}

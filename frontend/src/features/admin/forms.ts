import { z } from 'zod'
import type { ProjectDraft, RevisionConflictProblem } from '../../api/generated/types.gen'

export type ProjectFormValues = { referenceCode: string; title: string; abstract: string; academicYear: string; semester: '' | 'first' | 'second' | 'summer'; programVersionId: string; majorVersionId: string; courseVersionId: string }

export const projectDraftSchema = z.object({
  academicYear: z.string().refine((value) => value === '' || /^\d{4}$/.test(value), 'Academic year must contain four digits.'),
})

export function toProjectDraft(values: ProjectFormValues): ProjectDraft {
  return { reference_code: values.referenceCode || null, title: values.title || null, abstract: values.abstract || null, academic_year: values.academicYear ? Number(values.academicYear) : null, semester: values.semester || null, program_version_id: values.programVersionId || null, major_version_id: values.majorVersionId || null, course_version_id: values.courseVersionId || null }
}

export function preserveDraftOnConflict<T>(values: T, conflict: RevisionConflictProblem): { values: T; conflict: RevisionConflictProblem } {
  return { values, conflict }
}

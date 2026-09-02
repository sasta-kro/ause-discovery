import { describe, expect, it } from 'vitest'
import type { RevisionConflictProblem } from '../../api/generated/types.gen'
import { preserveDraftOnConflict, projectDraftSchema, toProjectDraft } from './forms'

describe('project draft handling', () => {
  const values = { referenceCode: '', title: '', abstract: '', academicYear: '', semester: '', programVersionId: '', majorVersionId: '', courseVersionId: '' } as const

  it('permits an incomplete draft while validating invalid year input', () => {
    expect(projectDraftSchema.safeParse(values).success).toBe(true)
    expect(projectDraftSchema.safeParse({ ...values, academicYear: 'twenty' }).success).toBe(false)
    expect(toProjectDraft(values).title).toBeNull()
  })

  it('preserves unsaved field values with the server conflict detail', () => {
    const conflict = { code: 'revision_conflict', current_revision: 3, type: '', title: '', status: 409, request_id: '' } as RevisionConflictProblem
    expect(preserveDraftOnConflict({ ...values, title: 'Unsaved title' }, conflict)).toMatchObject({ values: { title: 'Unsaved title' }, conflict: { current_revision: 3 } })
  })
})

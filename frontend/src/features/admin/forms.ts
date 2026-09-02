import { z } from 'zod'
import type { AdminProject, ParticipationInput, ProjectDraft, RevisionConflictProblem, TaxonomyAssignmentInput, TaxonomyDimension } from '../../api/generated/types.gen'

export type ProjectFormValues = {
  referenceCode: string
  title: string
  abstract: string
  academicYear: string
  semester: '' | 'first' | 'second' | 'summer'
  programVersionId: string
  majorVersionId: string
  courseVersionId: string
  studentPersonIds: string[]
  advisorPersonIds: string[]
  coAdvisorPersonIds: string[]
  committeePersonIds: string[]
  categoryTaxonomyIds: string[]
  platformTaxonomyIds: string[]
  domainTaxonomyIds: string[]
  topicTaxonomyIds: string[]
  technologyTaxonomyIds: string[]
}

type ProjectFormSource = Pick<AdminProject, 'reference_code' | 'title' | 'abstract' | 'academic_year' | 'semester' | 'program' | 'major' | 'course' | 'participations' | 'taxonomy'>

export const projectDraftSchema = z.object({
  academicYear: z.string().refine((value) => value === '' || /^\d{4}$/.test(value), 'Academic year must contain four digits.'),
})

export function toProjectDraft(values: ProjectFormValues): ProjectDraft {
  return {
    reference_code: values.referenceCode || null,
    title: values.title || null,
    abstract: values.abstract || null,
    academic_year: values.academicYear ? Number(values.academicYear) : null,
    semester: values.semester || null,
    program_version_id: values.programVersionId || null,
    major_version_id: values.majorVersionId || null,
    course_version_id: values.courseVersionId || null,
    participations: [
      ...participations(values.studentPersonIds, 'student'),
      ...participations(values.advisorPersonIds, 'advisor'),
      ...participations(values.coAdvisorPersonIds, 'co_advisor'),
      ...participations(values.committeePersonIds, 'committee_member'),
    ],
    taxonomy_values: [
      ...taxonomyAssignments(values.categoryTaxonomyIds),
      ...taxonomyAssignments(values.platformTaxonomyIds),
      ...taxonomyAssignments(values.domainTaxonomyIds),
      ...taxonomyAssignments(values.topicTaxonomyIds),
      ...taxonomyAssignments(values.technologyTaxonomyIds),
    ],
  }
}

export function toProjectFormValues(project?: Partial<ProjectFormSource>): ProjectFormValues {
  return {
    referenceCode: project?.reference_code ?? '',
    title: project?.title ?? '',
    abstract: project?.abstract ?? '',
    academicYear: project?.academic_year?.toString() ?? '',
    semester: project?.semester ?? '',
    programVersionId: project?.program?.id ?? '',
    majorVersionId: project?.major?.id ?? '',
    courseVersionId: project?.course?.id ?? '',
    studentPersonIds: personIDs(project, 'student'),
    advisorPersonIds: personIDs(project, 'advisor'),
    coAdvisorPersonIds: personIDs(project, 'co_advisor'),
    committeePersonIds: personIDs(project, 'committee_member'),
    categoryTaxonomyIds: taxonomyIDs(project, 'category'),
    platformTaxonomyIds: taxonomyIDs(project, 'platform'),
    domainTaxonomyIds: taxonomyIDs(project, 'domain'),
    topicTaxonomyIds: taxonomyIDs(project, 'topic'),
    technologyTaxonomyIds: taxonomyIDs(project, 'technology'),
  }
}

function participations(personIDs: string[] | undefined, role: ParticipationInput['role']): ParticipationInput[] {
  return (personIDs ?? []).map((personID, sortOrder) => ({ person_id: personID, role, sort_order: sortOrder }))
}

function taxonomyAssignments(taxonomyValueIDs: string[] | undefined): TaxonomyAssignmentInput[] {
  return (taxonomyValueIDs ?? []).map((taxonomyValueID, sortOrder) => ({ taxonomy_value_id: taxonomyValueID, sort_order: sortOrder }))
}

function personIDs(project: Partial<ProjectFormSource> | undefined, role: ParticipationInput['role']): string[] {
  return (project?.participations ?? []).filter((participation) => participation.role === role).sort((left, right) => left.sort_order - right.sort_order).map((participation) => participation.person.id)
}

function taxonomyIDs(project: Partial<ProjectFormSource> | undefined, dimension: TaxonomyDimension): string[] {
  return (project?.taxonomy ?? []).filter((value) => value.dimension === dimension).sort((left, right) => left.sort_order - right.sort_order).map((value) => value.id)
}

export function preserveDraftOnConflict<T>(values: T, conflict: RevisionConflictProblem): { values: T; conflict: RevisionConflictProblem } {
  return { values, conflict }
}

export function projectDeleteConfirmation(project: Pick<AdminProject, 'reference_code' | 'title'>): string {
  return project.reference_code ?? project.title ?? ''
}

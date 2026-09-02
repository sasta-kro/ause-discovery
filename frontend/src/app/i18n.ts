import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

export const englishResources = {
  translation: {
    brand: 'AUSE Discovery',
    subtitle: 'Historical senior project archive',
    nav: { search: 'Search', about: 'About', admin: 'Administration', projects: 'Projects', people: 'People', imports: 'Imports', searchMaintenance: 'Search maintenance', audit: 'Audit log', signOut: 'Sign out' },
    action: { search: 'Search projects', filter: 'Apply filters', clear: 'Clear filters', view: 'View', download: 'Download', saveDraft: 'Save draft', publish: 'Publish', delete: 'Delete', restore: 'Restore', cancel: 'Cancel', confirm: 'Confirm', edit: 'Edit', create: 'Create', reload: 'Reload current record', backToResults: 'Back to results', signIn: 'Sign in', uploadArtifact: 'Upload Artifact', updateArtifact: 'Update metadata', replaceArtifact: 'Replace file' },
    home: { eyebrow: 'Assumption University', heading: 'Discover the work that shaped a generation.', description: 'Search historical senior projects by topic, person, academic context, and classification.', searchLabel: 'Search the project archive', searchPlaceholder: 'Topic, title, person, or seven-digit student ID', browse: 'Browse the archive' },
    search: { title: 'Search projects', results: '{{count}} projects', noResults: 'No projects matched the current search.', loading: 'Searching the archive…', unavailable: 'Search is temporarily unavailable. Project pages remain available.', query: 'Search terms', sort: 'Sort results', relevance: 'Relevance', newest: 'Newest first', oldest: 'Oldest first', alphabetical: 'Title A to Z', filters: 'Filters', next: 'Load more', academic: 'Academic context', people: 'People', classification: 'Classification', availability: 'Artifact availability' },
    fields: { year: 'Academic year', semester: 'Semester', program: 'Program', major: 'Major', course: 'Course', advisor: 'Advisor', student: 'Student', category: 'Category', platform: 'Platform', domain: 'Domain', topic: 'Topic', technology: 'Technology', artifacts: 'Artifacts', report: 'Report available', slides: 'Slides available', sourceCode: 'Source code available', dataset: 'Dataset available', referenceCode: 'Reference code', title: 'Project title', abstract: 'Abstract', status: 'Status', displayName: 'Display name', studentId: 'Student ID', staffId: 'Staff ID', username: 'Username', password: 'Password', confirmDelete: 'Confirmation value', artifactType: 'Artifact type', file: 'Artifact file', replacementFile: 'Replacement file', first: 'First semester', second: 'Second semester', summer: 'Summer semester' },
    project: { title: 'Project details', people: 'People and roles', academic: 'Academic context', classifications: 'Classifications', artifactList: 'Available artifacts', noArtifacts: 'No active artifacts are available.', students: 'Students', advisors: 'Advisors', coAdvisors: 'Co-advisors', committee: 'Committee members' },
    person: { title: 'Person record', projects: 'Associated projects', noProjects: 'No associated published projects are available.' },
    legal: { aboutTitle: 'About AUSE Discovery', privacyTitle: 'Privacy policy', accessibilityTitle: 'Accessibility statement', termsTitle: 'Terms of use', placeholder: 'Approved institutional content will be published here.' },
    admin: { title: 'Administration', overview: 'Record maintenance workspace', loginTitle: 'Administrator sign in', loginDescription: 'Use an authorized local administrator account.', loginFailed: 'The credentials could not be verified.', sessionLoading: 'Checking administrator session…', forbidden: 'An active administrator session is required.', projectsTitle: 'Projects', peopleTitle: 'People', newProject: 'New project', newPerson: 'New person', draft: 'Draft', published: 'Published', deleted: 'Deleted', projectForm: 'Project record', personForm: 'Person record', coreMetadata: 'Core metadata', academicContext: 'Academic context', peopleAssignments: 'People and roles', taxonomyAssignments: 'Classifications', categories: 'Categories', platforms: 'Platforms', domains: 'Domains', topics: 'Topics', technologies: 'Technologies', artifactManagement: 'Artifacts', artifactHelp: 'Upload controlled Project files, update their labels, replace immutable versions, or change active status.', artifactProjectDeleted: 'Restore the Project before changing Artifact records.', draftHelp: 'Draft records may be incomplete. Publication requirements are validated by the service.', serverIssues: 'The service reported validation issues.', conflict: 'This record changed elsewhere. Unsaved changes remain in this form.', deleteTitle: 'Delete project', deleteDescription: 'Deletion hides this Project from public discovery and can be reversed later.', deletePrompt: 'Type {{value}} to confirm this reversible deletion.', restored: 'The Project was restored.', placeholder: 'This administrator area is ready for the related service increment.', publishHelp: 'Publish validates the complete Project record.' },
    artifact: { type: { report: 'Report', slides: 'Slides', source_code: 'Source code', proposal: 'Proposal', poster: 'Poster', dataset: 'Dataset', demo_video: 'Demo video', other: 'Other' } },
    feedback: { required: 'This field is required.', invalidStudentId: 'Student ID must contain exactly seven digits.', saved: 'The record was saved.', notFound: 'The requested record was not found.', unexpected: 'An unexpected error occurred.', artifactFailed: 'The Artifact operation failed. Reload the Project record before retrying.', loading: 'Loading…' },
    footer: { about: 'About', privacy: 'Privacy', accessibility: 'Accessibility', terms: 'Terms', contact: 'Contact information pending', credit: 'AUSE Discovery' },
    units: { megabytes: '{{count}} MB' },
  },
} as const

void i18n.use(initReactI18next).init({
  resources: { en: englishResources },
  lng: 'en',
  fallbackLng: 'en',
  interpolation: { escapeValue: false },
})

export default i18n

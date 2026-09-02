import { Component, type ReactNode, createContext, useContext, useEffect, useMemo, useState } from 'react'
import { I18nextProvider, useTranslation } from 'react-i18next'
import { QueryClient, QueryClientProvider, keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useForm, type UseFormRegister } from 'react-hook-form'
import { BrowserRouter, Link, NavLink, Navigate, Outlet, Route, Routes, useLocation, useNavigate, useParams, useSearchParams } from 'react-router'
import { createPerson, createProject, deleteProject, getAdminPerson, getAdminProject, getCatalogs, getCsrfToken, getPublicPerson, getPublicProject, getSession, listAdminPeople, listAdminProjects, login, logout, publishProject, replaceProject, restoreProject, searchProjects, updatePerson } from './api/generated/sdk.gen'
import type { AdminProject, CatalogsResponse, Problem, RevisionConflictProblem, SessionResponse, TaxonomyValue } from './api/generated/types.gen'
import i18n from './app/i18n'
import { configureApiClient, routerBasename } from './app/runtime'
import { highlightText } from './features/search/highlight'
import { parseSearchState, resetSearchCursor, serializeSearchState, type SearchState } from './features/search/state'
import { projectDraftSchema, toProjectDraft, type ProjectFormValues } from './features/admin/forms'
import styles from './App.module.css'

configureApiClient()

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: 1, staleTime: 30_000 } } })

async function responseData<T>(request: Promise<{ data: T }>): Promise<T> {
  return (await request).data
}

function isProblem(value: unknown): value is Problem {
  return typeof value === 'object' && value !== null && 'code' in value && 'status' in value
}

async function sessionRequest(): Promise<SessionResponse | null> {
  try {
    return await responseData(getSession({ throwOnError: true }))
  } catch (error) {
    if (isProblem(error) && error.status === 401) return null
    throw error
  }
}

type SessionContextValue = { session: SessionResponse | null | undefined; csrfToken: string | null; refresh: () => Promise<void>; signOut: () => Promise<void> }
const SessionContext = createContext<SessionContextValue | null>(null)

function SessionProvider({ children }: { children: ReactNode }) {
  const client = useQueryClient()
  const sessionQuery = useQuery({ queryKey: ['session'], queryFn: sessionRequest, retry: false })
  const [csrfToken, setCsrfToken] = useState<string | null>(null)

  useEffect(() => {
    if (!sessionQuery.data) {
      setCsrfToken(null)
      return
    }
    void responseData(getCsrfToken({ throwOnError: true })).then((result) => setCsrfToken(result.token)).catch(() => setCsrfToken(null))
  }, [sessionQuery.data])

  const value = useMemo<SessionContextValue>(() => ({
    session: sessionQuery.isPending ? undefined : (sessionQuery.data ?? null),
    csrfToken,
    refresh: async () => { await client.invalidateQueries({ queryKey: ['session'] }) },
    signOut: async () => {
      if (csrfToken) await responseData(logout({ headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true }))
      setCsrfToken(null)
      await client.setQueryData(['session'], null)
    },
  }), [client, csrfToken, sessionQuery.data, sessionQuery.isPending])
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
}

function useSession(): SessionContextValue {
  const value = useContext(SessionContext)
  if (!value) throw new Error('SessionProvider is required')
  return value
}

export function protectedRedirect(next: string): string {
  return `/admin/login?next=${encodeURIComponent(next)}`
}

function PageTitle({ title }: { title: string }) {
  const { t } = useTranslation()
  useEffect(() => { document.title = `${title} | ${t('brand')}` }, [t, title])
  return null
}

class ApplicationErrorBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false }
  static getDerivedStateFromError(): { failed: boolean } { return { failed: true } }
  render() {
    if (this.state.failed) return <div role="alert">{i18n.t('feedback.unexpected')}</div>
    return this.props.children
  }
}

function PublicLayout() {
  const { t } = useTranslation()
  return <div className={styles.page}>
    <a className={styles.skipLink} href="#main-content">{t('action.backToResults')}</a>
    <header className={styles.header}><div className={styles.headerInner}>
      <Link className={styles.brand} to="/">{t('brand')}</Link>
      <nav className={styles.navigation} aria-label={t('brand')}>
        <NavLink to="/search">{t('nav.search')}</NavLink><NavLink to="/about">{t('nav.about')}</NavLink><NavLink to="/admin">{t('nav.admin')}</NavLink>
      </nav>
    </div></header>
    <main id="main-content" className={styles.main}><Outlet /></main>
    <footer className={styles.footer}><div className={styles.footerInner}>
      <span>{t('footer.credit')}</span><nav className={styles.footerNav} aria-label={t('footer.credit')}>
        <Link to="/about">{t('footer.about')}</Link><Link to="/privacy">{t('footer.privacy')}</Link><Link to="/accessibility">{t('footer.accessibility')}</Link><Link to="/terms">{t('footer.terms')}</Link><span>{t('footer.contact')}</span>
      </nav>
    </div></footer>
  </div>
}

function HomePage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  return <section className={styles.hero}><PageTitle title={t('brand')} /><p className={styles.eyebrow}>{t('home.eyebrow')}</p><h1>{t('home.heading')}</h1><p className={styles.lede}>{t('home.description')}</p>
    <form className={styles.searchBox} onSubmit={(event) => { event.preventDefault(); navigate(`/search?${new URLSearchParams({ q: query }).toString()}`) }}>
      <label className="sr-only" htmlFor="home-search">{t('home.searchLabel')}</label><input id="home-search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('home.searchPlaceholder')} />
      <button className={styles.button} type="submit">{t('action.search')}</button>
    </form>
    <p><Link className={styles.secondaryButton} to="/search">{t('home.browse')}</Link></p>
  </section>
}

type FilterDefinition = { key: keyof SearchState; label: string; facet: keyof CatalogsResponse }
const filters: FilterDefinition[] = [
  { key: 'program_key', label: 'fields.program', facet: 'programs' }, { key: 'major_key', label: 'fields.major', facet: 'majors' }, { key: 'course_key', label: 'fields.course', facet: 'courses' },
  { key: 'category_key', label: 'fields.category', facet: 'taxonomy' }, { key: 'platform_key', label: 'fields.platform', facet: 'taxonomy' }, { key: 'domain_key', label: 'fields.domain', facet: 'taxonomy' }, { key: 'topic_key', label: 'fields.topic', facet: 'taxonomy' }, { key: 'technology_key', label: 'fields.technology', facet: 'taxonomy' },
]

function SearchPage() {
  const { t } = useTranslation()
  const [parameters, setParameters] = useSearchParams()
  const state = parseSearchState(parameters)
  const searchQuery = useQuery({ queryKey: ['search', state], queryFn: () => responseData(searchProjects({ query: state, throwOnError: true })), placeholderData: keepPreviousData })
  const catalogsQuery = useQuery({ queryKey: ['catalogs'], queryFn: () => responseData(getCatalogs({ throwOnError: true })) })
  const setState = (next: SearchState) => setParameters(serializeSearchState(resetSearchCursor(next)))
  const queryTerms = (state.q ?? '').split(/\s+/)
  return <section><PageTitle title={t('search.title')} /><div className={styles.pageHeader}><h1>{t('search.title')}</h1></div>
    <form className={styles.searchBox} onSubmit={(event) => { event.preventDefault(); setState(state) }}>
      <label className="sr-only" htmlFor="search-query">{t('search.query')}</label><input id="search-query" value={state.q ?? ''} onChange={(event) => setState({ ...state, q: event.target.value })} />
      <select aria-label={t('search.sort')} value={state.sort} onChange={(event) => setState({ ...state, sort: event.target.value as SearchState['sort'] })}><option value="relevance">{t('search.relevance')}</option><option value="newest">{t('search.newest')}</option><option value="oldest">{t('search.oldest')}</option><option value="title">{t('search.alphabetical')}</option></select>
      <button className={styles.button} type="submit">{t('action.search')}</button>
    </form>
    <div className={styles.searchLayout}><aside className={styles.filterPanel} aria-label={t('search.filters')}><h2>{t('search.filters')}</h2>
      <fieldset className={styles.filterGroup}><legend>{t('search.academic')}</legend><label className={styles.filterOption}>{t('fields.year')}<input type="number" value={state.academic_year ?? ''} onChange={(event) => setState({ ...state, academic_year: event.target.value ? Number(event.target.value) : undefined })} /></label>
        <label className={styles.filterOption}>{t('fields.semester')}<select value={state.semester ?? ''} onChange={(event) => setState({ ...state, semester: event.target.value ? event.target.value as SearchState['semester'] : undefined })}><option value="" /><option value="first">{t('fields.first')}</option><option value="second">{t('fields.second')}</option><option value="summer">{t('fields.summer')}</option></select></label>
      </fieldset>
      <fieldset className={styles.filterGroup}><legend>{t('search.people')}</legend><label className={styles.filterOption}>{t('fields.studentId')}<input inputMode="numeric" value={state.student_id ?? ''} onChange={(event) => setState({ ...state, student_id: event.target.value || undefined })} /></label></fieldset>
      <fieldset className={styles.filterGroup}><legend>{t('search.classification')}</legend>{filters.map((filter) => <FacetSelect key={filter.key} filter={filter} state={state} catalogs={catalogsQuery.data} setState={setState} />)}</fieldset>
      <fieldset className={styles.filterGroup}><legend>{t('search.availability')}</legend>{(['has_artifacts', 'has_report', 'has_slides', 'has_source_code', 'has_dataset'] as const).map((key) => <label className={styles.filterOption} key={key}><input type="checkbox" checked={state[key] ?? false} onChange={(event) => setState({ ...state, [key]: event.target.checked || undefined })} />{t(`fields.${key === 'has_artifacts' ? 'artifacts' : key.replace('has_', '').replace(/_([a-z])/g, (_, character: string) => character.toUpperCase())}`)}</label>)}</fieldset>
      <button className={styles.secondaryButton} type="button" onClick={() => setParameters(new URLSearchParams())}>{t('action.clear')}</button>
    </aside>
    <div><div className={styles.resultsHeader}><div><h2>{t('search.results', { count: searchQuery.data?.total ?? 0 })}</h2>{searchQuery.isFetching && <p role="status">{t('search.loading')}</p>}</div></div>
      {searchQuery.isError ? <p className={styles.error} role="alert">{t('search.unavailable')}</p> : null}
      {searchQuery.data?.items.length === 0 ? <p>{t('search.noResults')}</p> : <div className={styles.resultList}>{searchQuery.data?.items.map((result) => <article className={styles.result} key={result.id}><h2><Link to={`/projects/${result.id}`}>{highlightText(result.title, queryTerms)}</Link></h2><div className={styles.metadata}><span>{result.academic_year}</span><span>{t(`fields.${result.semester}`)}</span><span>{result.program.label}</span><span>{result.people.map((participation) => participation.person.display_name).join(', ')}</span></div><div className={styles.tags}>{[...result.categories, ...result.platforms].map((taxonomy) => <span className={styles.tag} key={taxonomy.id}>{taxonomy.labels.en ?? taxonomy.key}</span>)}</div></article>)}</div>}
      {searchQuery.data?.page.next_cursor ? <button className={styles.secondaryButton} onClick={() => setParameters(serializeSearchState({ ...state, cursor: searchQuery.data?.page.next_cursor ?? undefined }))}>{t('search.next')}</button> : null}
    </div></div>
  </section>
}

function FacetSelect({ filter, state, catalogs, setState }: { filter: FilterDefinition; state: SearchState; catalogs?: CatalogsResponse; setState: (state: SearchState) => void }) {
  const { t } = useTranslation()
  const selected = (state[filter.key] as string[] | undefined) ?? []
  const catalogKey = filter.facet
  const options = catalogKey === 'taxonomy'
    ? catalogs?.taxonomy.filter((value) => filter.key.replace('_key', '') === value.dimension) ?? []
    : (catalogs?.[catalogKey] ?? [])
  return <label className={styles.filterOption}>{t(filter.label)}<select multiple value={selected} onChange={(event) => setState({ ...state, [filter.key]: Array.from(event.currentTarget.selectedOptions, (option) => option.value) })}>{options.map((option) => <option key={option.id} value={option.key}>{'labels' in option ? option.labels.en ?? option.key : option.label}</option>)}</select></label>
}

function ProjectPage() {
  const { t } = useTranslation(); const { projectId = '' } = useParams()
  const projectQuery = useQuery({ queryKey: ['project', projectId], queryFn: () => responseData(getPublicProject({ path: { project_id: projectId }, throwOnError: true })) })
  if (projectQuery.isPending) return <p role="status">{t('feedback.loading')}</p>
  if (projectQuery.isError || !projectQuery.data) return <NotFound />
  const project = projectQuery.data
  return <article><PageTitle title={project.title} /><Link to="/search">{t('action.backToResults')}</Link><div className={styles.pageHeader}><p className={styles.eyebrow}>{project.reference_code}</p><h1>{project.title}</h1></div><div className={styles.detailGrid}><div><p className={styles.lede}>{project.abstract}</p><section className={styles.detailSection}><h2>{t('project.people')}</h2><People participations={project.participations} /></section><section className={styles.detailSection}><h2>{t('project.classifications')}</h2><Tags values={project.taxonomy} /></section><section className={styles.detailSection}><h2>{t('project.artifactList')}</h2>{project.artifacts.length ? <div className={styles.projectList}>{project.artifacts.map((artifact) => <div className={styles.panel} key={artifact.id}><strong>{artifact.display_name}</strong><div className={styles.metadata}><span>{artifact.artifact_type}</span><span>{formatBytes(artifact.byte_count, t)}</span></div><p className={styles.formActions}>{artifact.view_url ? <a className={styles.secondaryButton} href={artifact.view_url}>{t('action.view')}</a> : null}{artifact.download_url ? <a className={styles.button} href={artifact.download_url}>{t('action.download')}</a> : null}</p></div>)}</div> : <p>{t('project.noArtifacts')}</p>}</section></div><aside><section className={styles.detailSection}><h2>{t('project.academic')}</h2><dl className={styles.definitionList}><Definition label={t('fields.year')} value={String(project.academic_year)} /><Definition label={t('fields.semester')} value={t(`fields.${project.semester}`)} /><Definition label={t('fields.program')} value={project.program.label} /><Definition label={t('fields.major')} value={project.major?.label} /><Definition label={t('fields.course')} value={project.course.label} /></dl></section></aside></div></article>
}

function PersonPage() {
  const { t } = useTranslation(); const { personId = '' } = useParams()
  const personQuery = useQuery({ queryKey: ['person', personId], queryFn: () => responseData(getPublicPerson({ path: { person_id: personId }, throwOnError: true })) })
  if (personQuery.isPending) return <p role="status">{t('feedback.loading')}</p>
  if (personQuery.isError || !personQuery.data) return <NotFound />
  const person = personQuery.data
  return <article><PageTitle title={person.display_name} /><div className={styles.pageHeader}><p className={styles.eyebrow}>{person.student_id}</p><h1>{person.display_name}</h1></div><section><h2>{t('person.projects')}</h2>{person.projects.length ? <div className={styles.projectList}>{person.projects.map((project) => <article className={styles.result} key={project.id}><h3><Link to={`/projects/${project.id}`}>{project.title}</Link></h3><p className={styles.metadata}>{project.academic_year} · {t(`fields.${project.semester}`)} · {project.role}</p></article>)}</div> : <p>{t('person.noProjects')}</p>}</section></article>
}

function People({ participations }: { participations: Array<{ person: { id: string; display_name: string }; role: string }> }) {
  return <div className={styles.projectList}>{participations.map((participation) => <p key={`${participation.person.id}-${participation.role}`}><Link to={`/people/${participation.person.id}`}>{participation.person.display_name}</Link> <span className={styles.metadata}>{participation.role}</span></p>)}</div>
}
function Tags({ values }: { values: TaxonomyValue[] }) { return <div className={styles.tags}>{values.map((value) => <span className={styles.tag} key={value.id}>{value.labels.en ?? value.key}</span>)}</div> }
function Definition({ label, value }: { label: string; value?: string | null }) { return value ? <div><dt>{label}</dt><dd>{value}</dd></div> : null }
function formatBytes(bytes: number, translate: (key: string, options: { count: string }) => string): string { return translate('units.megabytes', { count: (bytes / 1024 / 1024).toFixed(bytes >= 10 * 1024 * 1024 ? 0 : 1) }) }

function StaticPage({ titleKey }: { titleKey: 'aboutTitle' | 'privacyTitle' | 'accessibilityTitle' | 'termsTitle' }) { const { t } = useTranslation(); return <section><PageTitle title={t(`legal.${titleKey}`)} /><div className={styles.pageHeader}><h1>{t(`legal.${titleKey}`)}</h1></div><p className={styles.lede}>{t('legal.placeholder')}</p></section> }
function NotFound() { const { t } = useTranslation(); return <section><PageTitle title={t('feedback.notFound')} /><h1>{t('feedback.notFound')}</h1></section> }

function AdminGuard() {
  const { t } = useTranslation(); const location = useLocation(); const { session } = useSession()
  if (session === undefined) return <p role="status">{t('admin.sessionLoading')}</p>
  if (!session) return <Navigate replace to={protectedRedirect(`${location.pathname}${location.search}`)} />
  return <AdminLayout />
}

function AdminLayout() {
  const { t } = useTranslation(); const navigate = useNavigate(); const { signOut } = useSession()
  return <section><PageTitle title={t('admin.title')} /><div className={styles.adminLayout}><nav className={styles.adminNav} aria-label={t('admin.title')}><NavLink end to="/admin">{t('admin.overview')}</NavLink><NavLink to="/admin/projects">{t('nav.projects')}</NavLink><NavLink to="/admin/people">{t('nav.people')}</NavLink><NavLink to="/admin/imports">{t('nav.imports')}</NavLink><NavLink to="/admin/search">{t('nav.searchMaintenance')}</NavLink><NavLink to="/admin/audit">{t('nav.audit')}</NavLink><button className={styles.secondaryButton} onClick={() => void signOut().then(() => navigate('/'))}>{t('nav.signOut')}</button></nav><div><Outlet /></div></div></section>
}

function LoginPage() {
  const { t } = useTranslation(); const navigate = useNavigate(); const [parameters] = useSearchParams(); const { session, refresh } = useSession(); const [error, setError] = useState(false)
  const form = useForm<{ username: string; password: string }>({ defaultValues: { username: '', password: '' } })
  if (session) return <Navigate to="/admin" replace />
  return <section className={styles.panel}><PageTitle title={t('admin.loginTitle')} /><h1>{t('admin.loginTitle')}</h1><p>{t('admin.loginDescription')}</p>{error ? <p className={styles.error} role="alert">{t('admin.loginFailed')}</p> : null}<form className={styles.form} onSubmit={form.handleSubmit(async (values) => { try { await responseData(login({ body: values, throwOnError: true })); await refresh(); navigate(parameters.get('next') || '/admin', { replace: true }) } catch { setError(true) } })}><FormField label={t('fields.username')} error={form.formState.errors.username?.message}><input {...form.register('username', { required: t('feedback.required') })} autoComplete="username" /></FormField><FormField label={t('fields.password')} error={form.formState.errors.password?.message}><input {...form.register('password', { required: t('feedback.required') })} autoComplete="current-password" type="password" /></FormField><button className={styles.button} type="submit">{t('action.signIn')}</button></form></section>
}

function AdminHome() { const { t } = useTranslation(); return <div><h1>{t('admin.title')}</h1><p className={styles.lede}>{t('admin.overview')}</p></div> }

function ProjectListPage() {
  const { t } = useTranslation(); const projectsQuery = useQuery({ queryKey: ['admin-projects'], queryFn: () => responseData(listAdminProjects({ query: { limit: 20 }, throwOnError: true })) })
  return <div><div className={styles.resultsHeader}><h1>{t('admin.projectsTitle')}</h1><Link className={styles.button} to="new">{t('admin.newProject')}</Link></div>{projectsQuery.isPending ? <p role="status">{t('feedback.loading')}</p> : null}{projectsQuery.data ? <div className={styles.tableWrap}><table className={styles.table}><thead><tr><th>{t('fields.title')}</th><th>{t('fields.status')}</th><th>{t('fields.year')}</th><th><span className="sr-only">{t('action.edit')}</span></th></tr></thead><tbody>{projectsQuery.data.items.map((project) => <tr key={project.id}><td>{project.title}</td><td>{t(`admin.${project.status}`)}</td><td>{project.academic_year}</td><td><Link to={`${project.id}/edit`}>{t('action.edit')}</Link></td></tr>)}</tbody></table></div> : null}</div>
}

function formValues(project?: AdminProject): ProjectFormValues { return { referenceCode: project?.reference_code ?? '', title: project?.title ?? '', abstract: project?.abstract ?? '', academicYear: project?.academic_year?.toString() ?? '', semester: project?.semester ?? '', programVersionId: project?.program?.id ?? '', majorVersionId: project?.major?.id ?? '', courseVersionId: project?.course?.id ?? '' } }

function ProjectFormPage({ isNew }: { isNew: boolean }) {
  const { t } = useTranslation(); const navigate = useNavigate(); const { projectId = '' } = useParams(); const { csrfToken } = useSession(); const client = useQueryClient(); const projectQuery = useQuery({ queryKey: ['admin-project', projectId], queryFn: () => responseData(getAdminProject({ path: { project_id: projectId }, throwOnError: true })), enabled: !isNew }); const catalogsQuery = useQuery({ queryKey: ['catalogs'], queryFn: () => responseData(getCatalogs({ throwOnError: true })) }); const [conflict, setConflict] = useState<RevisionConflictProblem | null>(null); const [serverIssues, setServerIssues] = useState<Array<{ field: string; message?: string }>>([])
  const form = useForm<ProjectFormValues>({ values: formValues(projectQuery.data) })
  const saveMutation = useMutation({ mutationFn: async (values: ProjectFormValues) => {
    const parsed = projectDraftSchema.safeParse(values); if (!parsed.success) throw parsed.error
    if (!csrfToken) throw new Error('CSRF token unavailable')
    const draft = toProjectDraft(values)
    if (isNew) return responseData(createProject({ body: draft, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true }))
    return responseData(replaceProject({ path: { project_id: projectId }, body: { expected_revision: projectQuery.data?.revision ?? 0, ...draft }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true }))
  }, onSuccess: (project) => { setConflict(null); setServerIssues([]); void client.invalidateQueries({ queryKey: ['admin-projects'] }); navigate(`/admin/projects/${project.id}/edit`, { replace: true }) }, onError: (error) => { if (isProblem(error) && error.code === 'revision_conflict') setConflict(error as unknown as RevisionConflictProblem); else if (isProblem(error) && 'issues' in error) setServerIssues((error as Problem & { issues: Array<{ field: string; message?: string }> }).issues) } })
  const publishMutation = useMutation({ mutationFn: async () => { if (!csrfToken || !projectQuery.data) throw new Error('CSRF token unavailable'); return responseData(publishProject({ path: { project_id: projectId }, body: { expected_revision: projectQuery.data.revision }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true })) }, onSuccess: (project) => { void client.setQueryData(['admin-project', project.id], project) }, onError: (error) => { if (isProblem(error) && 'issues' in error) setServerIssues((error as Problem & { issues: Array<{ field: string; message?: string }> }).issues) } })
  if (!isNew && projectQuery.isPending) return <p role="status">{t('feedback.loading')}</p>
  return <div><h1>{t('admin.projectForm')}</h1><p>{t('admin.draftHelp')}</p>{conflict ? <div className={styles.conflict} role="alert"><p>{t('admin.conflict')}</p><button className={styles.secondaryButton} onClick={() => { void client.invalidateQueries({ queryKey: ['admin-project', projectId] }); setConflict(null) }}>{t('action.reload')}</button></div> : null}{serverIssues.length ? <div className={styles.error} role="alert"><strong>{t('admin.serverIssues')}</strong><ul>{serverIssues.map((issue, index) => <li key={`${issue.field}-${index}`}>{issue.message ?? issue.field}</li>)}</ul></div> : null}<form className={styles.form} onSubmit={form.handleSubmit((values) => saveMutation.mutate(values))}><ProjectFields form={form} catalogs={catalogsQuery.data} /><div className={styles.formActions}><button className={styles.button} disabled={saveMutation.isPending} type="submit">{t('action.saveDraft')}</button>{!isNew ? <button className={styles.secondaryButton} disabled={publishMutation.isPending} type="button" onClick={() => publishMutation.mutate()}>{t('action.publish')}</button> : null}</div>{!isNew && projectQuery.data ? <DeleteRestoreControls project={projectQuery.data} /> : null}</form></div>
}

function ProjectFields({ form, catalogs }: { form: ReturnType<typeof useForm<ProjectFormValues>>; catalogs?: CatalogsResponse }) {
  const { t } = useTranslation()
  return <><section className={styles.detailSection}><h2>{t('admin.coreMetadata')}</h2><FormField label={t('fields.title')} error={form.formState.errors.title?.message}><input {...form.register('title')} /></FormField><FormField label={t('fields.referenceCode')} error={form.formState.errors.referenceCode?.message}><input {...form.register('referenceCode')} /></FormField><FormField label={t('fields.abstract')} error={form.formState.errors.abstract?.message}><textarea {...form.register('abstract')} /></FormField></section><section className={styles.detailSection}><h2>{t('admin.academicContext')}</h2><FormField label={t('fields.year')} error={form.formState.errors.academicYear?.message}><input inputMode="numeric" {...form.register('academicYear')} /></FormField><FormField label={t('fields.semester')}><select {...form.register('semester')}><option value="" /><option value="first">{t('fields.first')}</option><option value="second">{t('fields.second')}</option><option value="summer">{t('fields.summer')}</option></select></FormField><CatalogSelect label={t('fields.program')} register={form.register('programVersionId')} options={catalogs?.programs ?? []} /><CatalogSelect label={t('fields.major')} register={form.register('majorVersionId')} options={catalogs?.majors ?? []} /><CatalogSelect label={t('fields.course')} register={form.register('courseVersionId')} options={catalogs?.courses ?? []} /></section><section className={styles.detailSection}><h2>{t('admin.peopleAssignments')}</h2><p className={styles.placeholder}>{t('admin.placeholder')}</p></section><section className={styles.detailSection}><h2>{t('admin.taxonomyAssignments')}</h2><p className={styles.placeholder}>{t('admin.placeholder')}</p></section><section className={styles.detailSection}><h2>{t('admin.artifactManagement')}</h2><p className={styles.placeholder}>{t('admin.artifactHelp')}</p></section></>
}
function CatalogSelect({ label, register, options }: { label: string; register: ReturnType<UseFormRegister<ProjectFormValues>>; options: CatalogsResponse['programs'] }) { return <FormField label={label}><select {...register}><option value="" />{options.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}</select></FormField> }
function FormField({ label, error, children }: { label: string; error?: string; children: ReactNode }) { return <div className={styles.field}><span>{label}</span><label>{children}</label>{error ? <span className={styles.fieldError} role="alert">{error}</span> : null}</div> }

export function DeleteConfirmation({ onCancel, onConfirm }: { title: string; onCancel: () => void; onConfirm: () => void }) {
  const { t } = useTranslation(); const [confirmation, setConfirmation] = useState('')
  return <div className={styles.dialogBackdrop} role="presentation"><section className={styles.dialog} role="dialog" aria-modal="true" aria-labelledby="delete-title"><h2 id="delete-title">{t('admin.deleteTitle')}</h2><p>{t('admin.deleteDescription')}</p><p>{t('admin.deletePrompt')}</p><label className={styles.field}>{t('fields.confirmDelete')}<input value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></label><div className={styles.formActions}><button className={styles.dangerButton} disabled={confirmation !== 'DELETE'} onClick={onConfirm}>{t('action.delete')}</button><button className={styles.secondaryButton} onClick={onCancel}>{t('action.cancel')}</button></div></section></div>
}

function DeleteRestoreControls({ project }: { project: AdminProject }) {
  const { t } = useTranslation(); const { csrfToken } = useSession(); const client = useQueryClient(); const [confirming, setConfirming] = useState(false)
  const mutation = useMutation({ mutationFn: async (action: 'delete' | 'restore') => { if (!csrfToken) throw new Error('CSRF token unavailable'); const request = { path: { project_id: project.id }, body: { expected_revision: project.revision }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true as const }; return action === 'delete' ? responseData(deleteProject({ ...request, body: { ...request.body, confirmation: 'DELETE' } })) : responseData(restoreProject(request)) }, onSuccess: (nextProject) => { void client.setQueryData(['admin-project', project.id], nextProject); setConfirming(false) } })
  if (project.status === 'deleted') return <button className={styles.secondaryButton} type="button" onClick={() => mutation.mutate('restore')}>{t('action.restore')}</button>
  return <>{confirming ? <DeleteConfirmation title={project.title ?? ''} onCancel={() => setConfirming(false)} onConfirm={() => mutation.mutate('delete')} /> : null}<button className={styles.dangerButton} type="button" onClick={() => setConfirming(true)}>{t('action.delete')}</button></>
}

function PeopleListPage() {
  const { t } = useTranslation(); const navigate = useNavigate(); const peopleQuery = useQuery({ queryKey: ['admin-people'], queryFn: () => responseData(listAdminPeople({ query: { limit: 20 }, throwOnError: true })) }); const { csrfToken } = useSession(); const form = useForm<{ displayName: string; studentId: string; staffId: string }>({ defaultValues: { displayName: '', studentId: '', staffId: '' } }); const createMutation = useMutation({ mutationFn: async (value: { displayName: string; studentId: string; staffId: string }) => { if (!csrfToken) throw new Error('CSRF token unavailable'); if (value.studentId && !/^\d{7}$/.test(value.studentId)) throw new Error(t('feedback.invalidStudentId')); return responseData(createPerson({ body: { display_name: value.displayName, student_id: value.studentId || null, staff_id: value.staffId || null }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true })) }, onSuccess: (person) => navigate(`/admin/people/${person.id}`) })
  return <div><h1>{t('admin.peopleTitle')}</h1><section className={styles.panel}><h2>{t('admin.newPerson')}</h2><form className={styles.form} onSubmit={form.handleSubmit((values) => createMutation.mutate(values))}><FormField label={t('fields.displayName')} error={form.formState.errors.displayName?.message}><input {...form.register('displayName', { required: t('feedback.required') })} /></FormField><FormField label={t('fields.studentId')}><input {...form.register('studentId')} /></FormField><FormField label={t('fields.staffId')}><input {...form.register('staffId')} /></FormField><button className={styles.button} type="submit">{t('action.create')}</button></form></section>{peopleQuery.data ? <div className={styles.tableWrap}><table className={styles.table}><thead><tr><th>{t('fields.displayName')}</th><th>{t('fields.studentId')}</th><th /></tr></thead><tbody>{peopleQuery.data.items.map((person) => <tr key={person.id}><td>{person.display_name}</td><td>{person.student_id}</td><td><Link to={person.id}>{t('action.edit')}</Link></td></tr>)}</tbody></table></div> : null}</div>
}

function PersonFormPage() { const { t } = useTranslation(); const { personId = '' } = useParams(); const { csrfToken } = useSession(); const personQuery = useQuery({ queryKey: ['admin-person', personId], queryFn: () => responseData(getAdminPerson({ path: { person_id: personId }, throwOnError: true })) }); const form = useForm<{ displayName: string; studentId: string; staffId: string }>({ values: { displayName: personQuery.data?.display_name ?? '', studentId: personQuery.data?.student_id ?? '', staffId: personQuery.data?.staff_id ?? '' } }); const mutation = useMutation({ mutationFn: async (values: { displayName: string; studentId: string; staffId: string }) => { if (!csrfToken || !personQuery.data) throw new Error('CSRF token unavailable'); return responseData(updatePerson({ path: { person_id: personId }, body: { expected_revision: personQuery.data.revision, display_name: values.displayName, student_id: values.studentId || null, staff_id: values.staffId || null }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true })) } }); if (personQuery.isPending) return <p role="status">{t('feedback.loading')}</p>; return <div><h1>{t('admin.personForm')}</h1><form className={styles.form} onSubmit={form.handleSubmit((values) => mutation.mutate(values))}><FormField label={t('fields.displayName')}><input {...form.register('displayName')} /></FormField><FormField label={t('fields.studentId')}><input {...form.register('studentId')} /></FormField><FormField label={t('fields.staffId')}><input {...form.register('staffId')} /></FormField><button className={styles.button} type="submit">{t('action.saveDraft')}</button></form></div> }

function AdminPlaceholder() { const { t } = useTranslation(); return <div className={styles.placeholder}><p>{t('admin.placeholder')}</p></div> }

function Router() { return <Routes><Route element={<PublicLayout />}><Route index element={<HomePage />} /><Route path="search" element={<SearchPage />} /><Route path="projects/:projectId" element={<ProjectPage />} /><Route path="people/:personId" element={<PersonPage />} /><Route path="about" element={<StaticPage titleKey="aboutTitle" />} /><Route path="privacy" element={<StaticPage titleKey="privacyTitle" />} /><Route path="accessibility" element={<StaticPage titleKey="accessibilityTitle" />} /><Route path="terms" element={<StaticPage titleKey="termsTitle" />} /><Route path="*" element={<NotFound />} /></Route><Route path="admin/login" element={<LoginPage />} /><Route path="admin" element={<AdminGuard />}><Route index element={<AdminHome />} /><Route path="projects" element={<ProjectListPage />} /><Route path="projects/new" element={<ProjectFormPage isNew />} /><Route path="projects/:projectId/edit" element={<ProjectFormPage isNew={false} />} /><Route path="people" element={<PeopleListPage />} /><Route path="people/:personId" element={<PersonFormPage />} /><Route path="imports" element={<AdminPlaceholder />} /><Route path="imports/:batchId" element={<AdminPlaceholder />} /><Route path="search" element={<AdminPlaceholder />} /><Route path="audit" element={<AdminPlaceholder />} /></Route></Routes> }

export function App() { return <ApplicationErrorBoundary><I18nextProvider i18n={i18n}><BrowserRouter basename={routerBasename}><QueryClientProvider client={queryClient}><SessionProvider><Router /></SessionProvider></QueryClientProvider></BrowserRouter></I18nextProvider></ApplicationErrorBoundary> }

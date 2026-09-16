import { Component, type ReactNode, createContext, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { I18nextProvider, useTranslation } from 'react-i18next'
import { QueryClient, QueryClientProvider, keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useForm, type UseFormRegister, type UseFormReturn } from 'react-hook-form'
import { BrowserRouter, Link, NavLink, Navigate, Outlet, Route, Routes, useLocation, useNavigationType, useNavigate, useParams, useSearchParams } from 'react-router'
import { createProject, deleteArtifact, deleteProject, getAdminProject, getCatalogs, getCsrfToken, getPublicPerson, getPublicProject, getSession, listAdminPeople, login, logout, publishProject, replaceArtifact, replaceProject, restoreArtifact, restoreProject, searchProjects, updateArtifact, uploadArtifact } from './api/generated/sdk.gen'
import type { AdminPerson, AdminProject, Artifact, ArtifactType, CatalogsResponse, Problem, RevisionConflictProblem, SearchFacet, SearchFacets, SessionResponse, TaxonomyValue } from './api/generated/types.gen'
import i18n from './app/i18n'
import { isNotFoundFailure, isProblem } from './app/problem'
import { publicBasePath } from './app/public-base-path'
import { configureApiClient, routerBasename } from './app/runtime'
import { installSessionExpiryNotification } from './app/session-expiry'
import { highlightText } from './features/search/highlight'
import { FacetDisclosure, SelectedFilterChip, StudentIdDisclosure, projectIdentityVariant, projectInitials, type FilterChoice } from './features/search/filter-controls'
import { SearchSuggestionInput } from './features/search/suggestion-input'
import { applyFilterValue, buildSuggestionDefs, isSuggestionField, type SuggestionField } from './features/search/suggestions'
import { useDecodedLogo } from './decoded-logo'
import { displayedSort, parseSearchState, resetSearchCursor, serializeSearchState, type SearchState } from './features/search/state'
import { projectDeleteConfirmation, projectDraftSchema, toProjectDraft, toProjectFormValues, type ProjectFormValues } from './features/admin/forms'
import { AdminSearchMaintenance } from './features/admin/search-maintenance'
import { AdminImportReview, AdminImportUpload } from './features/admin/import-management'
import { AdminAuditLog } from './features/admin/audit-log'
import { AdminProjectList } from './features/admin/project-list'
import { AdminPersonForm } from './features/admin/person-form'
import { ProjectLogoManagement } from './features/admin/project-logo'
import { AdminPeopleList } from './features/admin/people-list'
import styles from './App.module.css'

configureApiClient()

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: 1, staleTime: 30_000 } } })

async function responseData<T>(request: Promise<{ data: T }>): Promise<T> {
  return (await request).data
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

export function SessionProvider({ children }: { children: ReactNode }) {
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
    if (this.state.failed) return <div className={styles.page} role="alert"><main id="main-content" className={styles.main}>
      <h1>{i18n.t('feedback.unexpected')}</h1>
      <p className={styles.lede}>{i18n.t('feedback.unexpectedHelp')}</p>
      <div className={styles.formActions}><a className={styles.button} href={publicBasePath}>{i18n.t('action.backHome')}</a><button className={styles.secondaryButton} type="button" onClick={() => window.location.reload()}>{i18n.t('action.reloadPage')}</button></div>
    </main></div>
    return this.props.children
  }
}

function AppFrame() {
  const { t } = useTranslation()
  const location = useLocation()
  const navigationType = useNavigationType()
  const mainRef = useRef<HTMLElement>(null)
  // The ref starts at the current pathname, so the initial effect run and any
  // development StrictMode replay of it see no pathname change and do nothing.
  const previousPathname = useRef(location.pathname)
  useEffect(() => {
    const pathnameChanged = location.pathname !== previousPathname.current
    previousPathname.current = location.pathname
    if (!pathnameChanged) return
    // Main keeps route focus with preventScroll so focusing can never move
    // the viewport below the site header.
    mainRef.current?.focus({ preventScroll: true })
    // A new pathname begins at document position zero. History POP keeps the
    // browser's restored scroll position, and same-path search changes are
    // application state, not new pages, so they never reach this effect.
    if (navigationType !== 'POP') window.scrollTo(0, 0)
    // The navigation type is read from the render that changed the pathname;
    // it must not key the effect, or same-path state transitions would
    // re-focus main.
  }, [location.pathname])
  return <div className={styles.page}>
    <a className={styles.skipLink} href="#main-content">{t('action.skipToMainContent')}</a>
    <header className={styles.header}><div className={styles.headerInner}>
      <Link className={styles.brand} onClick={() => { if (location.pathname === '/') window.scrollTo(0, 0) }} to="/" aria-label={t('brand')}>
        <img className={styles.brandLogo} src={`${publicBasePath}ause-discover-logo-header.svg`} alt="" aria-hidden="true" />
      </Link>
      <nav className={styles.navigation} aria-label={t('nav.primary')}>
        <NavLink to="/search">{t('nav.search')}</NavLink><NavLink to="/about">{t('nav.about')}</NavLink><NavLink className={styles.adminNavigationLink} to="/admin">{t('nav.admin')}</NavLink>
      </nav>
    </div></header>
    <main id="main-content" ref={mainRef} tabIndex={-1} className={`${styles.main} ${location.pathname === '/' ? styles.homeMain : ''} ${location.pathname === '/search' ? styles.searchMain : ''}`}><Outlet /></main>
    <footer className={styles.footer}><div className={styles.footerInner}>
      <span>{t('footer.creditPrefix')} <a href="https://github.com/sasta-kro" rel="noopener noreferrer" target="_blank">{t('footer.creditName')}</a></span><nav className={styles.footerNav} aria-label={t('nav.footer')}>
        <Link to="/about">{t('footer.about')}</Link><Link to="/privacy">{t('footer.privacy')}</Link><Link to="/accessibility">{t('footer.accessibility')}</Link><Link to="/terms">{t('footer.terms')}</Link><Link to="/contact">{t('footer.contact')}</Link>
      </nav>
    </div></footer>
  </div>
}

function RequestFailure() {
  const { t } = useTranslation()
  return <div><PageTitle title={t('feedback.requestFailed')} /><h1>{t('feedback.requestFailedTitle')}</h1><p className={styles.error} role="alert">{t('feedback.requestFailed')}</p><p className={styles.formActions}><Link className={styles.secondaryButton} to="/">{t('action.backHome')}</Link></p></div>
}

function NotFound() {
  const { t } = useTranslation()
  return <div><PageTitle title={t('feedback.notFound')} /><h1>{t('feedback.notFound')}</h1><p className={styles.lede}>{t('feedback.notFoundHelp')}</p>
    <p className={styles.formActions}><Link className={styles.button} to="/">{t('action.backHome')}</Link><Link className={styles.secondaryButton} to="/search">{t('action.search')}</Link></p></div>
}

function HomePage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  return <section className={styles.hero}><PageTitle title={t('brand')} /><p className={styles.eyebrow}>{t('home.eyebrow')}</p><h1>{t('home.heading')}</h1><p className={styles.lede}>{t('home.description')}</p>
    <div className={styles.homeActions}>
      <Link className={`${styles.button} ${styles.browsePrimary}`} to="/search">{t('home.browse')}</Link>
      <p className={styles.homeSearchPrompt}>{t('home.searchPrompt')}</p>
      <form className={styles.searchBox} onSubmit={(event) => { event.preventDefault(); navigate(`/search?${new URLSearchParams({ q: query }).toString()}`) }}>
        <label className="sr-only" htmlFor="home-search">{t('home.searchLabel')}</label><input id="home-search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('home.searchPlaceholder')} />
        <button className={styles.secondaryButton} type="submit">{t('action.search')}</button>
      </form>
    </div>
  </section>
}

type ArrayFilterKey = 'program_key' | 'major_key' | 'course_key' | 'person_id' | 'advisor_id' | 'category_key' | 'platform_key' | 'domain_key' | 'topic_key' | 'technology_key'
type CatalogFilterDefinition = { key: Exclude<ArrayFilterKey, 'person_id' | 'advisor_id'>; label: string; facet: keyof SearchFacets; catalog: keyof CatalogsResponse; dimension?: string }
// Major remains supported by catalog and search contracts but is intentionally paused in public discovery.
const academicFilters: CatalogFilterDefinition[] = [
  { key: 'program_key', label: 'fields.program', facet: 'programs', catalog: 'programs' },
  { key: 'course_key', label: 'fields.course', facet: 'courses', catalog: 'courses' },
]
// Topic remains supported by storage and import contracts but is intentionally inactive in interfaces and keyword matching.
const classificationFilters: CatalogFilterDefinition[] = [
  { key: 'category_key', label: 'fields.category', facet: 'categories', catalog: 'taxonomy', dimension: 'category' },
  { key: 'platform_key', label: 'fields.platform', facet: 'platforms', catalog: 'taxonomy', dimension: 'platform' },
  { key: 'domain_key', label: 'fields.domain', facet: 'domains', catalog: 'taxonomy', dimension: 'domain' },
  { key: 'technology_key', label: 'fields.technology', facet: 'technologies', catalog: 'taxonomy', dimension: 'technology' },
]
const filters = [...academicFilters, ...classificationFilters]

const availabilityFilters = [
  { key: 'has_artifacts', label: 'fields.artifacts' },
  { key: 'has_report', label: 'fields.report' },
  { key: 'has_slides', label: 'fields.slides' },
  { key: 'has_source_code', label: 'fields.sourceCode' },
  { key: 'has_dataset', label: 'fields.dataset' },
] as const

function withCurrentChoice(options: FilterChoice[], value: string | undefined): FilterChoice[] {
  if (!value || options.some((option) => option.value === value)) return options
  return [{ value, label: value }, ...options]
}

function facetChoices(facets: SearchFacet[] | undefined): FilterChoice[] {
  return (facets ?? []).map((facet) => ({ value: facet.key, label: facet.label ?? facet.key, count: facet.count }))
}

function catalogChoices(filter: CatalogFilterDefinition, catalogs: CatalogsResponse | undefined, facets: SearchFacets | undefined): FilterChoice[] {
  const counts = new Map((facets?.[filter.facet] ?? []).map((facet) => [facet.key, facet.count]))
  const source = filter.catalog === 'taxonomy'
    ? catalogs?.taxonomy.filter((value) => value.dimension === filter.dimension) ?? []
    : catalogs?.[filter.catalog] ?? []
  return source.map((option) => ({
    value: option.key,
    label: 'labels' in option ? option.labels.en ?? option.key : option.label,
    count: counts.get(option.key),
  }))
}

function SearchPage() {
  const { t } = useTranslation()
  const [parameters, setParameters] = useSearchParams()
  const state = parseSearchState(parameters)
  const [draft, setDraft] = useState(state.q ?? '')
  const [cursorHistory, setCursorHistory] = useState<Array<string | undefined>>([])
  const searchQuery = useQuery({ queryKey: ['search', state], queryFn: () => responseData(searchProjects({ query: { ...state, limit: 20 }, throwOnError: true })), placeholderData: keepPreviousData })
  const catalogsQuery = useQuery({ queryKey: ['catalogs'], queryFn: () => responseData(getCatalogs({ throwOnError: true })) })
  useEffect(() => { setDraft(state.q ?? '') }, [state.q])
  const setState = (next: SearchState) => {
    setDraft(next.q ?? '')
    setCursorHistory([])
    setParameters(serializeSearchState(resetSearchCursor(next)))
  }
  const goToCursor = (cursor: string | undefined) => setParameters(serializeSearchState({ ...state, cursor }))
  const nextPage = () => {
    const nextCursor = searchQuery.data?.page.next_cursor
    if (!nextCursor) return
    setCursorHistory((previous) => [...previous, state.cursor])
    goToCursor(nextCursor)
  }
  const previousPage = () => {
    if (!cursorHistory.length) return
    goToCursor(cursorHistory[cursorHistory.length - 1])
    setCursorHistory((previous) => previous.slice(0, -1))
  }
  const toggleArrayFilter = (key: ArrayFilterKey, value: string) => {
    const selected = (state[key] as string[] | undefined) ?? []
    if (selected.includes(value)) {
      const next = selected.filter((item) => item !== value)
      setState({ ...state, [key]: next.length ? next : undefined })
      return
    }
    // Adding routes through the same shared operation suggestions use;
    // dimensions outside the suggestion model keep the plain append
    // behavior.
    if (isSuggestionField(key)) {
      setState(applyFilterValue(state, key, value))
      return
    }
    setState({ ...state, [key]: [...selected, value] })
  }
  const clearFilters = () => setState({ q: state.q, limit: state.limit, sort: state.sort })
  const catalogFilterChoices = new Map(filters.map((filter) => [filter.key, catalogChoices(filter, catalogsQuery.data, searchQuery.data?.facets)]))
  const peopleChoices = facetChoices(searchQuery.data?.facets.people)
  const advisorChoices = facetChoices(searchQuery.data?.facets.advisors)
  const yearChoices = withCurrentChoice(facetChoices(searchQuery.data?.facets.academic_years), state.academic_year ? String(state.academic_year) : undefined)
  const semesterCounts = new Map((searchQuery.data?.facets.semesters ?? []).map((facet) => [facet.key, facet.count]))
  const semesterChoices: FilterChoice[] = [
    { value: 'first', label: t('fields.first'), count: semesterCounts.get('first') },
    { value: 'second', label: t('fields.second'), count: semesterCounts.get('second') },
    { value: 'summer', label: t('fields.summer'), count: semesterCounts.get('summer') },
  ]
  // Suggestion sources use the same choice data as the left panel, in panel
  // dimension order: year, semester, academic catalogs, People, Advisor,
  // then the classification dimensions. Person facets refresh from the
  // latest successful Search response and keepPreviousData holds the prior
  // facet set stable during a short refetch.
  const suggestionDefs = useMemo(() => buildSuggestionDefs([
    { stateField: 'academic_year', dimensionLabel: t('fields.year'), cardinality: 'scalar', choices: yearChoices.map((choice) => ({ value: choice.value, label: choice.label })) },
    { stateField: 'semester', dimensionLabel: t('fields.semester'), cardinality: 'scalar', choices: semesterChoices.map((choice) => ({ value: choice.value, label: choice.label })) },
    ...academicFilters.map((filter) => ({ stateField: filter.key as SuggestionField, dimensionLabel: t(filter.label), cardinality: 'multiple' as const, choices: (catalogFilterChoices.get(filter.key) ?? []).map((choice) => ({ value: choice.value, label: choice.label })) })),
    { stateField: 'person_id', dimensionLabel: t('search.people'), cardinality: 'multiple', choices: peopleChoices.map((choice) => ({ value: choice.value, label: choice.label })) },
    { stateField: 'advisor_id', dimensionLabel: t('fields.advisor'), cardinality: 'multiple', choices: advisorChoices.map((choice) => ({ value: choice.value, label: choice.label })) },
    ...classificationFilters.map((filter) => ({ stateField: filter.key as SuggestionField, dimensionLabel: t(filter.label), cardinality: 'multiple' as const, choices: (catalogFilterChoices.get(filter.key) ?? []).map((choice) => ({ value: choice.value, label: choice.label })) })),
  ]), [t, yearChoices, semesterChoices, catalogFilterChoices, peopleChoices, advisorChoices])
  const activeFilters: Array<{ id: string; label: string; remove: () => void }> = []
  if (state.academic_year) activeFilters.push({ id: 'academic-year', label: `${t('fields.year')}: ${state.academic_year}`, remove: () => setState({ ...state, academic_year: undefined }) })
  if (state.semester) activeFilters.push({ id: 'semester', label: semesterChoices.find((option) => option.value === state.semester)?.label ?? state.semester, remove: () => setState({ ...state, semester: undefined }) })
  if (state.student_id) activeFilters.push({ id: 'student-id', label: `${t('fields.studentId')}: ${state.student_id}`, remove: () => setState({ ...state, student_id: undefined }) })
  for (const filter of filters) {
    const selected = (state[filter.key] as string[] | undefined) ?? []
    for (const value of selected) activeFilters.push({
      id: `${filter.key}-${value}`,
      label: `${t(filter.label)}: ${catalogFilterChoices.get(filter.key)?.find((option) => option.value === value)?.label ?? value}`,
      remove: () => toggleArrayFilter(filter.key, value),
    })
  }
  for (const value of state.person_id ?? []) activeFilters.push({
    id: `person-${value}`,
    label: `${t('search.people')}: ${peopleChoices.find((option) => option.value === value)?.label ?? value}`,
    remove: () => toggleArrayFilter('person_id', value),
  })
  for (const value of state.advisor_id ?? []) activeFilters.push({
    id: `advisor-${value}`,
    label: `${t('fields.advisor')}: ${advisorChoices.find((option) => option.value === value)?.label ?? value}`,
    remove: () => toggleArrayFilter('advisor_id', value),
  })
  for (const filter of availabilityFilters) if (state[filter.key]) activeFilters.push({
    id: filter.key,
    label: t(filter.label),
    remove: () => setState({ ...state, [filter.key]: undefined }),
  })
  const queryTerms = (state.q ?? '').split(/\s+/)
  const pending = searchQuery.isFetching
  return <section className={styles.searchPage}><PageTitle title={t('search.title')} /><div className={styles.searchPageHeader}><h1>{t('search.title')}</h1></div>
    <form className={`${styles.searchBox} ${styles.searchToolbar}`} onSubmit={(event) => { event.preventDefault(); setState({ ...state, q: draft || undefined }) }}>
      <label className="sr-only" htmlFor="search-query">{t('search.query')}</label>
      <SearchSuggestionInput
        applied={state}
        placeholder={t('search.placeholder')}
        suggestionsDefs={suggestionDefs}
        value={draft}
        onAccept={(suggestion, remaining) => {
          setDraft(remaining)
          setState(applyFilterValue({ ...state, q: remaining || undefined }, suggestion.stateField, suggestion.value))
        }}
        onChange={setDraft}
        onSearch={() => setState({ ...state, q: draft || undefined })}
      />
      <select aria-label={t('search.sort')} value={displayedSort(state)} onChange={(event) => setState({ ...state, sort: event.target.value as SearchState['sort'] })}><option value="relevance">{t('search.relevance')}</option><option value="newest">{t('search.newest')}</option><option value="oldest">{t('search.oldest')}</option><option value="title">{t('search.alphabetical')}</option></select>
      <button className={styles.searchPrimaryButton} type="submit" disabled={pending}>{t('action.search')}</button>
    </form>
    <div className={styles.searchLayout}><aside className={styles.filterPanel} aria-label={t('search.filters')}><div className={styles.filterPanelHeader}><h2>{t('search.refine')}</h2><button className={styles.clearFiltersButton} disabled={!activeFilters.length} type="button" onClick={clearFilters}>{t('search.clearAll')}</button></div>
      <div className={styles.selectedFilters} aria-label={t('search.selectedFilters')}><h3>{t('search.selectedFilters')}</h3>{activeFilters.length ? <div className={styles.selectedFilterList}>{activeFilters.map((filter) => <SelectedFilterChip key={filter.id} label={filter.label} onRemove={filter.remove} />)}</div> : <p>{t('search.noSelectedFilters')}</p>}</div>
      {catalogsQuery.isPending ? <p role="status">{t('search.filtersLoading')}</p> : null}
      {catalogsQuery.isError ? <p className={styles.error} role="alert">{t('search.filtersUnavailable')}</p> : null}
      <FacetDisclosure label={t('fields.year')} options={yearChoices} order="year-desc" searchLabel={t('search.findYear')} selected={state.academic_year ? [String(state.academic_year)] : []} onToggle={(value) => setState(state.academic_year === Number(value) ? { ...state, academic_year: undefined } : applyFilterValue(state, 'academic_year', value))} />
      <FacetDisclosure label={t('fields.semester')} options={semesterChoices} order="fixed" selected={state.semester ? [state.semester] : []} onToggle={(value) => setState(state.semester === value ? { ...state, semester: undefined } : applyFilterValue(state, 'semester', value))} />
      {academicFilters.map((filter) => <FacetDisclosure key={filter.key} label={t(filter.label)} options={catalogFilterChoices.get(filter.key) ?? []} order="toggleable" searchLabel={t('search.findFilter', { filter: t(filter.label).toLocaleLowerCase() })} selected={(state[filter.key] as string[] | undefined) ?? []} onToggle={(value) => toggleArrayFilter(filter.key, value)} />)}
      <FacetDisclosure label={t('search.people')} options={peopleChoices} order="toggleable" searchLabel={t('search.findPeople')} selected={state.person_id ?? []} onToggle={(value) => toggleArrayFilter('person_id', value)} />
      <FacetDisclosure label={t('fields.advisor')} options={advisorChoices} order="toggleable" searchLabel={t('search.findAdvisor')} selected={state.advisor_id ?? []} onToggle={(value) => toggleArrayFilter('advisor_id', value)} />
      <StudentIdDisclosure value={state.student_id} onApply={(value) => setState({ ...state, student_id: value })} />
      {classificationFilters.map((filter) => <FacetDisclosure defaultOpen={filter.key === 'technology_key'} key={filter.key} label={t(filter.label)} options={catalogFilterChoices.get(filter.key) ?? []} order="toggleable" searchLabel={t('search.findFilter', { filter: t(filter.label).toLocaleLowerCase() })} selected={(state[filter.key] as string[] | undefined) ?? []} onToggle={(value) => toggleArrayFilter(filter.key, value)} />)}
      <FacetDisclosure label={t('search.availability')} options={availabilityFilters.map((filter) => ({ value: filter.key, label: t(filter.label) }))} order="fixed" selected={availabilityFilters.filter((filter) => state[filter.key]).map((filter) => filter.key)} onToggle={(value) => setState({ ...state, [value]: state[value as typeof availabilityFilters[number]['key']] ? undefined : true })} />
      <div className={styles.filterActions}><button className={styles.searchPrimaryButton} type="button" onClick={() => void searchQuery.refetch()}>{t('action.filter')}</button><button className={styles.secondaryButton} disabled={!activeFilters.length} type="button" onClick={clearFilters}>{t('action.clear')}</button></div>
    </aside>
    <div className={styles.searchResults}><div className={styles.resultsHeader}><div><h2>{t('search.results', { count: searchQuery.data?.total ?? 0 })}</h2>{searchQuery.isPending ? <p role="status">{t('search.loading')}</p> : pending ? <p role="status">{t('search.updating')}</p> : null}</div></div>
      {searchQuery.isError ? <p className={styles.error} role="alert">{t('search.unavailable')}</p> : null}
      {!searchQuery.isPending && searchQuery.data?.items.length === 0 ? <p>{t('search.noResults')}</p> : <div className={styles.resultList}>{searchQuery.data?.items.map((result, index) => <article className={styles.searchResult} key={result.id}>
        <ProjectIdentity eager={index < eagerLogoResults} logoUrl={result.logo_url} title={result.title} variantClass={projectIdentityVariant(index) === 'purple' ? styles.projectIdentityPurple : styles.projectIdentityRed} />
        <div className={styles.resultBody}><h2><Link to={`/projects/${result.id}`}>{highlightText(result.title, queryTerms)}</Link></h2>
          <div className={styles.metadata}><span>{result.academic_year}</span><span>{t(`fields.${result.semester}`)}</span><span>{result.program.label}</span>{result.people.length ? <span>{result.people.map((participation) => participation.person.display_name).join(', ')}</span> : null}</div>
          {excerptOf(result.highlights, queryTerms)}
          <div className={styles.tags}>{[...result.categories, ...result.platforms].map((taxonomy) => <span className={styles.tag} key={taxonomy.id}>{taxonomy.labels.en ?? taxonomy.key}</span>)}</div>
        </div>
      </article>)}</div>}
      <div className={styles.formActions}>
        <button className={styles.secondaryButton} disabled={pending || !cursorHistory.length} type="button" onClick={previousPage}>{t('search.previous')}</button>
        <button className={styles.secondaryButton} disabled={pending || !searchQuery.data?.page.next_cursor} type="button" onClick={nextPage}>{t('search.next')}</button>
      </div>
    </div></div>
  </section>
}

// The first search results are the eager Logo cohort; every later result
// uses native lazy loading.
const eagerLogoResults = 4

export function ProjectDetailIdentity({ title, logoUrl, variantClass }: { title: string; logoUrl?: string | null; variantClass?: string }) {
  const { status, imageProps } = useDecodedLogo(logoUrl)
  const ready = status === 'ready'
  // The header Logo is above the fold and may be the page's largest image,
  // so it stays eager with high fetch priority; the decode still defers the
  // reveal until the image is fully prepared.
  return <div aria-hidden="true" className={`${styles.pageHeaderIdentity} ${ready ? styles.pageHeaderIdentityImage : variantClass ?? styles.pageHeaderIdentityPurple}`}>
    <span className={`${styles.pageHeaderInitials} ${ready ? styles.initialsConcealed : ''}`}>{projectInitials(title)}</span>
    {logoUrl && status !== 'failed' ? <img alt="" className={`${styles.pageHeaderLogo} ${ready ? styles.logoRevealed : styles.logoConcealed}`} decoding="async" fetchPriority="high" loading="eager" src={logoUrl} {...imageProps} /> : null}
  </div>
}

export function ProjectIdentity({ title, logoUrl, variantClass, eager = false }: { title: string; logoUrl?: string | null; variantClass?: string; eager?: boolean }) {
  const { status, imageProps } = useDecodedLogo(logoUrl)
  const ready = status === 'ready'
  // The first search results are likely visible in the initial viewport and
  // stay eager at default priority; every later result defers to native
  // lazy loading at low priority so Logos never compete with the document.
  return <div aria-hidden="true" className={`${styles.projectIdentity} ${variantClass ?? ''} ${ready ? styles.projectIdentityImage : ''}`}>
    <span className={`${styles.projectIdentityInitials} ${ready ? styles.initialsConcealed : ''}`}>{projectInitials(title)}</span>
    {logoUrl && status !== 'failed' ? <img alt="" className={`${styles.projectIdentityLogo} ${ready ? styles.logoRevealed : styles.logoConcealed}`} decoding="async" fetchPriority={eager ? undefined : 'low'} loading={eager ? 'eager' : 'lazy'} src={logoUrl} {...imageProps} /> : null}
  </div>
}

function excerptOf(highlights: Array<{ field: string; value: string }>, queryTerms: string[]) {
  const excerpt = highlights.find((highlight) => highlight.field === 'abstract')
  if (!excerpt) return null
  return <p className={styles.excerpt}>{highlightText(excerpt.value, queryTerms)}</p>
}

export function ProjectRepositories({ links }: { links: Array<{ url: string; primary: boolean; availability: string; checked_at: string }> }) {
  const { t } = useTranslation()
  if (!links.length) return null
  return <section className={styles.detailSection}>
    <h2>{t('project.repositories')}</h2>
    <div className={styles.repositoryList}>
      {links.map((link) => <div className={styles.panel} key={link.url}>
        <a className={styles.repositoryLink} href={link.url} rel="noopener noreferrer" target="_blank">{link.url}</a>
        <p className={styles.metadata}>
          {links.length > 1 && link.primary ? <span>{t('project.repositoryPrimary')}</span> : null}
          <span className={link.availability === 'accessible' ? styles.repositoryAvailabilityAccessible : undefined}>{t(`project.availability.${link.availability}`)}</span>
          <span>{t('project.lastChecked', { date: new Date(link.checked_at).toLocaleDateString() })}</span>
        </p>
      </div>)}
    </div>
  </section>
}

function ProjectPage() {
  const { t } = useTranslation(); const { projectId = '' } = useParams()
  const projectQuery = useQuery({ queryKey: ['project', projectId], queryFn: () => responseData(getPublicProject({ path: { project_id: projectId }, throwOnError: true })) })
  if (projectQuery.isPending) return <p role="status">{t('feedback.loading')}</p>
  if (projectQuery.isError || !projectQuery.data) return isNotFoundFailure(projectQuery.error) ? <NotFound /> : <RequestFailure />
  const project = projectQuery.data
  return <article><PageTitle title={project.title} /><Link to="/search">{t('action.backToResults')}</Link><div className={styles.pageHeader}><div className={styles.pageHeaderWithIdentity}><ProjectDetailIdentity logoUrl={project.logo_url} title={project.title} /><h1>{project.title}</h1></div></div><div className={styles.detailGrid}><div><p className={styles.lede}>{project.abstract}</p><section className={styles.detailSection}><h2>{t('project.people')}</h2><People participations={project.participations} /></section><section className={styles.detailSection}><h2>{t('project.classifications')}</h2><ClassificationGroups values={project.taxonomy} /></section><section className={styles.detailSection}><h2>{t('project.artifactList')}</h2>{project.artifacts.length ? <div className={styles.projectList}>{project.artifacts.map((artifact) => <div className={styles.panel} key={artifact.id}><strong>{artifact.display_name}</strong><div className={styles.metadata}><span>{t(`artifact.type.${artifact.artifact_type}`)}</span><span>{formatBytes(artifact.byte_count, t)}</span></div><p className={styles.formActions}>{artifact.view_url ? <a className={styles.secondaryButton} href={artifact.view_url} rel="noopener noreferrer" target="_blank">{t('action.view')}</a> : null}{artifact.download_url ? <a className={styles.button} href={artifact.download_url}>{t('action.download')}</a> : null}</p></div>)}</div> : <p>{t('project.noArtifacts')}</p>}</section><ProjectRepositories links={project.repository_links} /></div><aside><section className={styles.detailSection}><h2>{t('project.academic')}</h2><dl className={styles.definitionList}><Definition label={t('fields.year')} value={String(project.academic_year)} /><Definition label={t('fields.semester')} value={t(`fields.${project.semester}`)} /><Definition label={t('fields.program')} value={project.program.label} /><Definition label={t('fields.major')} value={project.major?.label} /><Definition label={t('fields.course')} value={project.course.label} /></dl></section></aside></div></article>
}

function PersonPage() {
  const { t } = useTranslation(); const { personId = '' } = useParams()
  const personQuery = useQuery({ queryKey: ['person', personId], queryFn: () => responseData(getPublicPerson({ path: { person_id: personId }, throwOnError: true })) })
  if (personQuery.isPending) return <p role="status">{t('feedback.loading')}</p>
  if (personQuery.isError || !personQuery.data) return isNotFoundFailure(personQuery.error) ? <NotFound /> : <RequestFailure />
  const person = personQuery.data
  return <article><PageTitle title={person.display_name} /><div className={styles.pageHeader}>{person.student_id ? <p className={styles.eyebrow}>{person.student_id}</p> : null}<h1>{person.display_name}</h1></div><section><h2>{t('person.projects')}</h2>{person.projects.length ? <div className={styles.projectList}>{person.projects.map((project) => <article className={styles.result} key={project.id}><h3><Link to={`/projects/${project.id}`}>{project.title}</Link></h3><p className={styles.metadata}>{project.academic_year} · {t(`fields.${project.semester}`)} · {t(`roles.${project.role}`)}</p></article>)}</div> : <p>{t('person.noProjects')}</p>}</section></article>
}

function People({ participations }: { participations: Array<{ person: { id: string; display_name: string }; role: string }> }) {
  const { t } = useTranslation()
  return <div className={styles.projectList}>{participations.map((participation) => <p key={`${participation.person.id}-${participation.role}`}><Link to={`/people/${participation.person.id}`}>{participation.person.display_name}</Link> <span className={styles.metadata}>{t(`roles.${participation.role}`)}</span></p>)}</div>
}
function Tags({ values }: { values: TaxonomyValue[] }) { return <div className={styles.tags}>{values.map((value) => <span className={styles.tag} key={value.id}>{value.labels.en ?? value.key}</span>)}</div> }
const publicClassificationDimensions = ['category', 'platform', 'domain', 'technology'] as const
function ClassificationGroups({ values }: { values: TaxonomyValue[] }) {
  const { t } = useTranslation()
  const groups = publicClassificationDimensions.map((dimension) => ({ dimension, values: values.filter((value) => value.dimension === dimension) })).filter((group) => group.values.length)
  if (!groups.length) return <p>{t('project.noClassifications')}</p>
  return <div className={styles.classificationGroups}>{groups.map((group) => <section className={styles.classificationGroup} key={group.dimension}><h3>{t(`fields.${group.dimension}`)}</h3><Tags values={group.values} /></section>)}</div>
}
function Definition({ label, value }: { label: string; value?: string | null }) { return value ? <div><dt>{label}</dt><dd>{value}</dd></div> : null }
function formatBytes(bytes: number, translate: (key: string, options: { count: string }) => string): string { return translate('units.megabytes', { count: (bytes / 1024 / 1024).toFixed(bytes >= 10 * 1024 * 1024 ? 0 : 1) }) }

function InformationalPage({ titleKey, bodyKey }: { titleKey: 'privacyTitle' | 'accessibilityTitle' | 'termsTitle' | 'contactTitle'; bodyKey: 'privacyBody' | 'accessibilityBody' | 'termsBody' | 'contactBody' }) {
  const { t } = useTranslation()
  return <section className={styles.prose}><PageTitle title={t(`legal.${titleKey}`)} /><div className={styles.pageHeader}><h1>{t(`legal.${titleKey}`)}</h1></div>
    <p className={styles.lede}>{t(`legal.${bodyKey}`)}</p>
  </section>
}

function AboutPage() {
  const { t } = useTranslation()
  return <section className={styles.prose}><PageTitle title={t('legal.aboutTitle')} /><div className={styles.pageHeader}><h1>{t('legal.aboutTitle')}</h1></div>
    <p className={styles.lede}>{t('legal.aboutBody')}</p>
    <section className={styles.aboutSection}>
      <h2>{t('legal.aboutDevelopmentTitle')}</h2>
      <p>{t('legal.aboutDevelopmentBody')}</p>
      <p className={styles.developerByline}><span>{t('footer.creditName')}</span><span className={styles.developerRole}>{t('legal.aboutDeveloperRole')}</span><a href="https://github.com/sasta-kro" rel="noopener noreferrer" target="_blank">{t('legal.aboutProfileLink')}</a></p>
    </section>
    <section className={styles.aboutSection}>
      <h2>{t('legal.aboutContentTitle')}</h2>
      <p>{t('legal.aboutContentBody')}</p>
    </section>
  </section>
}

function AdminGuard() {
  const { t } = useTranslation(); const location = useLocation(); const { session } = useSession()
  if (session === undefined) return <p role="status">{t('admin.sessionLoading')}</p>
  if (!session) return <Navigate replace to={protectedRedirect(`${location.pathname}${location.search}`)} />
  return <AdminLayout />
}

function AdminLayout() {
  const { t } = useTranslation(); const navigate = useNavigate(); const { signOut } = useSession()
  const [signingOut, setSigningOut] = useState(false)
  const [signOutFailed, setSignOutFailed] = useState(false)
  const handleSignOut = async () => {
    setSigningOut(true); setSignOutFailed(false)
    try {
      await signOut()
    } catch {
      setSigningOut(false); setSignOutFailed(true)
      return
    }
    navigate('/')
  }
  return <div><div className={styles.adminLayout}><nav className={styles.adminNav} aria-label={t('admin.title')}><NavLink end to="/admin">{t('admin.overview')}</NavLink><NavLink to="/admin/projects">{t('nav.projects')}</NavLink><NavLink to="/admin/people">{t('nav.people')}</NavLink><NavLink to="/admin/imports">{t('nav.imports')}</NavLink><NavLink to="/admin/search">{t('nav.searchMaintenance')}</NavLink><NavLink to="/admin/audit">{t('nav.audit')}</NavLink><button className={styles.secondaryButton} disabled={signingOut} onClick={() => void handleSignOut()}>{signingOut ? t('feedback.working') : t('nav.signOut')}</button></nav><div><Outlet /></div></div>{signOutFailed ? <p className={styles.error} role="alert">{t('admin.signOutFailed')}</p> : null}</div>
}

function LoginPage() {
  const { t } = useTranslation(); const navigate = useNavigate(); const [parameters] = useSearchParams(); const { session, refresh } = useSession(); const [error, setError] = useState(false)
  const form = useForm<{ username: string; password: string }>({ defaultValues: { username: '', password: '' } })
  const [submitting, setSubmitting] = useState(false)
  if (session) return <Navigate to="/admin" replace />
  return <section className={styles.panel}><PageTitle title={t('admin.loginTitle')} /><h1>{t('admin.loginTitle')}</h1><p>{t('admin.loginDescription')}</p>{error ? <p className={styles.error} role="alert">{t('admin.loginFailed')}</p> : null}<form className={styles.form} onSubmit={form.handleSubmit(async (values) => {
    setSubmitting(true); setError(false)
    try {
      await responseData(login({ body: values, throwOnError: true }))
      await refresh()
      navigate(parameters.get('next') || '/admin', { replace: true })
    } catch {
      setError(true)
    } finally {
      setSubmitting(false)
    }
  })}><FormField label={t('fields.username')} error={form.formState.errors.username?.message}><input {...form.register('username', { required: t('feedback.required') })} autoComplete="username" /></FormField><FormField label={t('fields.password')} error={form.formState.errors.password?.message}><input {...form.register('password', { required: t('feedback.required') })} autoComplete="current-password" type="password" /></FormField><button className={styles.button} disabled={submitting} type="submit">{t('action.signIn')}</button></form><p className={styles.sessionNotice}>{t('admin.sessionCookieNotice')}</p></section>
}

function AdminHome() { const { t } = useTranslation(); return <div><PageTitle title={t('admin.overviewTitle')} /><h1>{t('admin.title')}</h1><p className={styles.lede}>{t('admin.overview')}</p></div> }

function ProjectFormPage({ isNew }: { isNew: boolean }) {
  const { t } = useTranslation(); const navigate = useNavigate(); const { projectId = '' } = useParams(); const { csrfToken } = useSession(); const client = useQueryClient(); const projectQuery = useQuery({ queryKey: ['admin-project', projectId], queryFn: () => responseData(getAdminProject({ path: { project_id: projectId }, throwOnError: true })), enabled: !isNew }); const catalogsQuery = useQuery({ queryKey: ['catalogs'], queryFn: () => responseData(getCatalogs({ throwOnError: true })) }); const peopleQuery = useQuery({ queryKey: ['admin-people', 'project-form'], queryFn: () => responseData(listAdminPeople({ query: { limit: 100 }, throwOnError: true })) }); const [conflict, setConflict] = useState<RevisionConflictProblem | null>(null); const [serverIssues, setServerIssues] = useState<Array<{ field: string; message?: string }>>([]); const [generalFailure, setGeneralFailure] = useState(false)
  const form = useForm<ProjectFormValues>({ defaultValues: toProjectFormValues() })
  const loadedRecordSignature = useRef('')
  useEffect(() => {
    const project = projectQuery.data
    if (!project) return
    const signature = `${project.id}:${project.revision}`
    if (loadedRecordSignature.current !== signature) {
      loadedRecordSignature.current = signature
      form.reset(toProjectFormValues(project))
    }
  }, [form, projectQuery.data])
  const saveMutation = useMutation({ mutationFn: async (values: ProjectFormValues) => {
    const parsed = projectDraftSchema.safeParse(values); if (!parsed.success) throw parsed.error
    if (!csrfToken) throw new Error('CSRF token unavailable')
    const draft = toProjectDraft(values)
    if (isNew) return responseData(createProject({ body: draft, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true }))
    return responseData(replaceProject({ path: { project_id: projectId }, body: { expected_revision: projectQuery.data?.revision ?? 0, ...draft }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true }))
  }, onSuccess: (project) => { setConflict(null); setServerIssues([]); setGeneralFailure(false); void client.setQueryData(['admin-project', project.id], project); void client.invalidateQueries({ queryKey: ['admin-projects'] }); navigate(`/admin/projects/${project.id}/edit`, { replace: true }) }, onError: (error) => {
    if (isProblem(error) && error.code === 'revision_conflict') { setConflict(error as unknown as RevisionConflictProblem); return }
    if (isProblem(error) && 'issues' in error) { setServerIssues((error as Problem & { issues: Array<{ field: string; message?: string }> }).issues); return }
    setGeneralFailure(true)
  } })
  const publishMutation = useMutation({ mutationFn: async () => { if (!csrfToken || !projectQuery.data) throw new Error('CSRF token unavailable'); return responseData(publishProject({ path: { project_id: projectId }, body: { expected_revision: projectQuery.data.revision }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true })) }, onSuccess: (project) => { setGeneralFailure(false); void client.setQueryData(['admin-project', project.id], project) }, onError: (error) => {
    if (isProblem(error) && 'issues' in error) { setServerIssues((error as Problem & { issues: Array<{ field: string; message?: string }> }).issues); return }
    if (isProblem(error) && error.code === 'revision_conflict') { setConflict(error as unknown as RevisionConflictProblem); return }
    setGeneralFailure(true)
  } })
  if (!isNew && projectQuery.isPending) return <p role="status">{t('feedback.loading')}</p>
  if (!isNew && projectQuery.isError) return isNotFoundFailure(projectQuery.error) ? <NotFound /> : <RequestFailure />
  if (catalogsQuery.isPending || peopleQuery.isPending) return <p role="status">{t('admin.loadingForm')}</p>
  if (catalogsQuery.isError || peopleQuery.isError) {
    return <div><PageTitle title={isNew ? t('admin.newProject') : t('admin.projectForm')} /><h1>{t('admin.projectForm')}</h1>
      <p className={styles.error} role="alert">{catalogsQuery.isError ? t('admin.catalogFailed') : t('admin.peopleFailed')}</p>
      <p className={styles.formActions}><button className={styles.secondaryButton} type="button" onClick={() => { void catalogsQuery.refetch(); void peopleQuery.refetch() }}>{t('action.retry')}</button></p>
    </div>
  }
  return <div><PageTitle title={isNew ? t('admin.newProject') : t('admin.projectForm')} /><h1>{t('admin.projectForm')}</h1><p>{t('admin.draftHelp')}</p>
    {conflict ? <div className={styles.conflict} role="alert"><p>{t('admin.conflict')}</p><button className={styles.secondaryButton} onClick={() => { void client.invalidateQueries({ queryKey: ['admin-project', projectId] }); setConflict(null) }}>{t('action.reload')}</button></div> : null}
    {serverIssues.length ? <div className={styles.error} role="alert"><strong>{t('admin.serverIssues')}</strong><ul>{serverIssues.map((issue, index) => <li key={`${issue.field}-${index}`}>{issue.message ?? issue.field}</li>)}</ul></div> : null}
    {generalFailure ? <p className={styles.error} role="alert">{t('admin.projectSaveFailed')}</p> : null}
    {saveMutation.isSuccess && !saveMutation.isPending ? <p role="status">{t('feedback.saved')}</p> : null}
    {publishMutation.isSuccess && !publishMutation.isPending ? <p role="status">{t('admin.publishedStatus')}</p> : null}
    <form className={styles.form} onSubmit={form.handleSubmit((values) => saveMutation.mutate(values))}><ProjectFields form={form} catalogs={catalogsQuery.data} people={peopleQuery.data?.items ?? []} /><div className={styles.formActions}><button className={styles.button} disabled={saveMutation.isPending || !csrfToken} type="submit">{t('action.saveDraft')}</button>{!isNew ? <button className={styles.secondaryButton} disabled={publishMutation.isPending || !csrfToken} type="button" onClick={() => publishMutation.mutate()}>{t('action.publish')}</button> : null}</div>{!isNew && projectQuery.data ? <DeleteRestoreControls project={projectQuery.data} /> : null}</form>{!isNew && projectQuery.data ? <ProjectLogoManagement csrfToken={csrfToken} disabled={projectQuery.data.status === 'deleted'} project={projectQuery.data} /> : null}{!isNew && projectQuery.data ? <ArtifactManagement projectId={projectQuery.data.id} projectRevision={projectQuery.data.revision} artifacts={projectQuery.data.artifacts} csrfToken={csrfToken} disabled={projectQuery.data.status === 'deleted'} /> : null}</div>
}

function ProjectFields({ form, catalogs, people }: { form: UseFormReturn<ProjectFormValues>; catalogs?: CatalogsResponse; people: AdminPerson[] }) {
  const { t } = useTranslation()
  return <><section className={styles.detailSection}><h2>{t('admin.coreMetadata')}</h2><FormField label={t('fields.title')} error={form.formState.errors.title?.message}><input {...form.register('title')} /></FormField><FormField label={t('fields.referenceCode')} error={form.formState.errors.referenceCode?.message}><input {...form.register('referenceCode')} /></FormField><FormField label={t('fields.abstract')} error={form.formState.errors.abstract?.message}><textarea {...form.register('abstract')} /></FormField></section><section className={styles.detailSection}><h2>{t('admin.academicContext')}</h2><FormField label={t('fields.year')} error={form.formState.errors.academicYear?.message}><input inputMode="numeric" {...form.register('academicYear')} /></FormField><FormField label={t('fields.semester')}><select {...form.register('semester')}><option value="" /><option value="first">{t('fields.first')}</option><option value="second">{t('fields.second')}</option><option value="summer">{t('fields.summer')}</option></select></FormField><CatalogSelect label={t('fields.program')} register={form.register('programVersionId')} options={catalogs?.programs ?? []} /><CatalogSelect label={t('fields.major')} register={form.register('majorVersionId')} options={catalogs?.majors ?? []} /><CatalogSelect label={t('fields.course')} register={form.register('courseVersionId')} options={catalogs?.courses ?? []} /></section><ProjectAssignmentFields form={form} people={people} taxonomy={catalogs?.taxonomy ?? []} /></>
}

const artifactTypes: ArtifactType[] = ['report', 'slides', 'source_code', 'proposal', 'poster', 'dataset', 'demo_video', 'other']

type ArtifactUploadValues = { artifactType: ArtifactType; displayName: string; files: FileList }

export function ArtifactManagement({ projectId, projectRevision, artifacts, csrfToken, disabled = false }: { projectId: string; projectRevision: number; artifacts: Artifact[]; csrfToken: string | null; disabled?: boolean }) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const form = useForm<ArtifactUploadValues>({ defaultValues: { artifactType: 'report', displayName: '' } })
  const refreshProject = async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: ['admin-project', projectId] }),
      client.invalidateQueries({ queryKey: ['admin-projects'] }),
      client.invalidateQueries({ queryKey: ['project', projectId] }),
    ])
  }
  const uploadMutation = useMutation({
    mutationFn: async (values: ArtifactUploadValues) => {
      if (!csrfToken || !values.files?.[0]) throw new Error('Artifact upload prerequisites are unavailable')
      return responseData(uploadArtifact({ path: { project_id: projectId }, body: { expected_project_revision: projectRevision, artifact_type: values.artifactType, display_name: values.displayName, file: values.files[0] }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true }))
    },
    onSuccess: async () => { form.reset(); await refreshProject() },
  })
  return <section className={styles.detailSection}><h2>{t('admin.artifactManagement')}</h2><p>{t('admin.artifactHelp')}</p>{disabled ? <p className={styles.notice}>{t('admin.artifactProjectDeleted')}</p> : <form className={styles.artifactUpload} onSubmit={form.handleSubmit((values) => uploadMutation.mutate(values))}><FormField label={t('fields.artifactType')}><select {...form.register('artifactType')}>{artifactTypes.map((artifactType) => <option key={artifactType} value={artifactType}>{t(`artifact.type.${artifactType}`)}</option>)}</select></FormField><FormField label={t('fields.displayName')} error={form.formState.errors.displayName?.message}><input {...form.register('displayName', { required: t('feedback.required') })} /></FormField><FormField label={t('fields.file')} error={form.formState.errors.files?.message}><input type="file" {...form.register('files', { required: t('feedback.required') })} /></FormField><button className={styles.button} disabled={uploadMutation.isPending || !csrfToken} type="submit">{t('action.uploadArtifact')}</button></form>}{uploadMutation.isError ? <p className={styles.error} role="alert">{t('feedback.artifactFailed')}</p> : null}{uploadMutation.isSuccess && !uploadMutation.isPending ? <p role="status">{t('feedback.saved')}</p> : null}<div className={styles.artifactAdminList}>{artifacts.length ? artifacts.map((artifact) => <ArtifactEditor key={artifact.id} artifact={artifact} csrfToken={csrfToken} disabled={disabled} refreshProject={refreshProject} />) : <p>{t('project.noArtifacts')}</p>}</div></section>
}

function ArtifactEditor({ artifact, csrfToken, disabled, refreshProject }: { artifact: Artifact; csrfToken: string | null; disabled: boolean; refreshProject: () => Promise<void> }) {
  const { t } = useTranslation()
  const metadataForm = useForm<{ artifactType: ArtifactType; displayName: string }>({ values: { artifactType: artifact.artifact_type, displayName: artifact.display_name } })
  const replacementForm = useForm<{ files: FileList }>()
  const updateMutation = useMutation({ mutationFn: async (values: { artifactType: ArtifactType; displayName: string }) => {
    if (!csrfToken) throw new Error('CSRF token unavailable')
    return responseData(updateArtifact({ path: { artifact_id: artifact.id }, body: { expected_revision: artifact.revision, artifact_type: values.artifactType, display_name: values.displayName }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true }))
  }, onSuccess: refreshProject })
  const replaceMutation = useMutation({ mutationFn: async (values: { files: FileList }) => {
    if (!csrfToken || !values.files?.[0]) throw new Error('Artifact replacement prerequisites are unavailable')
    return responseData(replaceArtifact({ path: { artifact_id: artifact.id }, body: { expected_revision: artifact.revision, file: values.files[0] }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true }))
  }, onSuccess: async () => { replacementForm.reset(); await refreshProject() } })
  const lifecycleMutation = useMutation({ mutationFn: async (operation: 'delete' | 'restore') => {
    if (!csrfToken) throw new Error('CSRF token unavailable')
    const request = { path: { artifact_id: artifact.id }, body: { expected_revision: artifact.revision }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true as const }
    return operation === 'delete' ? responseData(deleteArtifact(request)) : responseData(restoreArtifact(request))
  }, onSuccess: refreshProject })
  const failed = updateMutation.isError || replaceMutation.isError || lifecycleMutation.isError
  const pending = updateMutation.isPending || replaceMutation.isPending || lifecycleMutation.isPending
  return <section className={styles.artifactEditor} role="group" aria-label={artifact.display_name}><div className={styles.resultsHeader}><div><strong>{artifact.display_name}</strong><div className={styles.metadata}><span>{t(`artifact.type.${artifact.artifact_type}`)}</span><span>{formatBytes(artifact.byte_count, t)}</span><span>{t(`admin.${artifact.status}`)}</span><span>{artifact.original_filename}</span></div></div><div className={styles.formActions}>{!disabled && artifact.view_url ? <a className={styles.secondaryButton} href={artifact.view_url} rel="noopener noreferrer" target="_blank">{t('action.view')}</a> : null}{!disabled && artifact.download_url ? <a className={styles.secondaryButton} href={artifact.download_url}>{t('action.download')}</a> : null}</div></div>{artifact.status === 'active' ? <><form className={styles.artifactControls} onSubmit={metadataForm.handleSubmit((values) => updateMutation.mutate(values))}><FormField label={t('fields.artifactType')}><select {...metadataForm.register('artifactType')}>{artifactTypes.map((artifactType) => <option key={artifactType} value={artifactType}>{t(`artifact.type.${artifactType}`)}</option>)}</select></FormField><FormField label={t('fields.displayName')}><input {...metadataForm.register('displayName', { required: true })} /></FormField><button className={styles.secondaryButton} disabled={disabled || updateMutation.isPending || !csrfToken} type="submit">{t('action.updateArtifact')}</button></form><form className={styles.artifactControls} onSubmit={replacementForm.handleSubmit((values) => replaceMutation.mutate(values))}><FormField label={t('fields.replacementFile')}><input type="file" {...replacementForm.register('files', { required: true })} /></FormField><button className={styles.secondaryButton} disabled={disabled || replaceMutation.isPending || !csrfToken} type="submit">{t('action.replaceArtifact')}</button></form><button className={styles.dangerButton} disabled={disabled || lifecycleMutation.isPending || !csrfToken} type="button" onClick={() => lifecycleMutation.mutate('delete')}>{t('action.delete')}</button></> : <button className={styles.secondaryButton} disabled={disabled || lifecycleMutation.isPending || !csrfToken} type="button" onClick={() => lifecycleMutation.mutate('restore')}>{t('action.restore')}</button>}{pending ? <p role="status">{t('feedback.working')}</p> : null}{failed ? <p className={styles.error} role="alert">{t('feedback.artifactFailed')}</p> : null}</section>
}

export function ProjectAssignmentFields({ form, people, taxonomy }: { form: UseFormReturn<ProjectFormValues>; people: AdminPerson[]; taxonomy: TaxonomyValue[] }) {
  const { t } = useTranslation()
  const personOptions = people.map((person) => ({ id: person.id, label: person.student_id ? `${person.display_name} (${person.student_id})` : person.display_name }))
  const taxonomyOptions = (dimension: TaxonomyValue['dimension']) => taxonomy.filter((value) => value.dimension === dimension).map((value) => ({ id: value.id, label: value.labels.en ?? value.key }))
  return <><section className={styles.detailSection}><h2>{t('admin.peopleAssignments')}</h2><div className={styles.assignmentGrid}><AssignmentSelect label={t('project.students')} name="studentPersonIds" options={personOptions} form={form} /><AssignmentSelect label={t('project.advisors')} name="advisorPersonIds" options={personOptions} form={form} /><AssignmentSelect label={t('project.coAdvisors')} name="coAdvisorPersonIds" options={personOptions} form={form} /><AssignmentSelect label={t('project.committee')} name="committeePersonIds" options={personOptions} form={form} /></div></section><section className={styles.detailSection}><h2>{t('admin.taxonomyAssignments')}</h2><div className={styles.assignmentGrid}><AssignmentSelect label={t('admin.categories')} name="categoryTaxonomyIds" options={taxonomyOptions('category')} form={form} /><AssignmentSelect label={t('admin.platforms')} name="platformTaxonomyIds" options={taxonomyOptions('platform')} form={form} /><AssignmentSelect label={t('admin.domains')} name="domainTaxonomyIds" options={taxonomyOptions('domain')} form={form} /><AssignmentSelect label={t('admin.technologies')} name="technologyTaxonomyIds" options={taxonomyOptions('technology')} form={form} /></div></section></>
}

type AssignmentFieldName = 'studentPersonIds' | 'advisorPersonIds' | 'coAdvisorPersonIds' | 'committeePersonIds' | 'categoryTaxonomyIds' | 'platformTaxonomyIds' | 'domainTaxonomyIds' | 'topicTaxonomyIds' | 'technologyTaxonomyIds'

function AssignmentSelect({ label, name, options, form }: { label: string; name: AssignmentFieldName; options: Array<{ id: string; label: string }>; form: UseFormReturn<ProjectFormValues> }) {
  return <FormField label={label}><select multiple size={Math.min(Math.max(options.length, 3), 8)} {...form.register(name)}>{options.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}</select></FormField>
}
function CatalogSelect({ label, register, options }: { label: string; register: ReturnType<UseFormRegister<ProjectFormValues>>; options: CatalogsResponse['programs'] }) { return <FormField label={label}><select {...register}><option value="" />{options.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}</select></FormField> }
function FormField({ label, error, children }: { label: string; error?: string; children: ReactNode }) { return <div className={styles.field}><label><span>{label}</span>{children}</label>{error ? <span className={styles.fieldError} role="alert">{error}</span> : null}</div> }

export function DeleteConfirmation({ confirmationValue, onCancel, onConfirm, pending = false }: { confirmationValue: string; onCancel: () => void; onConfirm: () => void; pending?: boolean }) {
  const { t } = useTranslation(); const [confirmation, setConfirmation] = useState('')
  const inputReference = useRef<HTMLInputElement>(null)
  const dialogRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    inputReference.current?.focus()
    return () => previousFocus?.focus()
  }, [])
  const trapFocus = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      if (pending) return
      event.preventDefault()
      onCancel()
      return
    }
    if (event.key !== 'Tab' || !dialogRef.current) return
    const focusable = dialogRef.current.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled])')
    if (!focusable.length) return
    const first = focusable[0]
    const last = focusable[focusable.length - 1]
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }
  return <div className={styles.dialogBackdrop} onPointerDown={(event) => { if (!pending && event.target === event.currentTarget) onCancel() }} role="presentation"><section aria-describedby="delete-description" aria-labelledby="delete-title" className={styles.dialog} id="delete-dialog" onKeyDown={trapFocus} ref={dialogRef} role="dialog" aria-modal="true"><h2 id="delete-title">{t('admin.deleteTitle')}</h2><p id="delete-description">{t('admin.deleteDescription')}</p><p>{t('admin.deletePrompt', { value: confirmationValue })}</p><label className={styles.field}>{t('fields.confirmDelete')}<input onChange={(event) => setConfirmation(event.target.value)} ref={inputReference} value={confirmation} /></label><div className={styles.formActions}><button className={styles.dangerButton} disabled={pending || confirmation !== confirmationValue} onClick={onConfirm} type="button">{t('action.delete')}</button><button className={styles.secondaryButton} disabled={pending} onClick={onCancel} type="button">{t('action.cancel')}</button></div></section></div>
}

function DeleteRestoreControls({ project }: { project: AdminProject }) {
  const { t } = useTranslation(); const { csrfToken } = useSession(); const client = useQueryClient(); const [confirming, setConfirming] = useState(false); const [conflict, setConflict] = useState(false); const [reloading, setReloading] = useState(false); const [reloadFailed, setReloadFailed] = useState(false)
  const conflictRevision = useRef(0)
  const confirmationValue = projectDeleteConfirmation(project)
  const mutation = useMutation({ mutationFn: async (action: 'delete' | 'restore') => { if (!csrfToken) throw new Error('CSRF token unavailable'); const request = { path: { project_id: project.id }, body: { expected_revision: project.revision }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true as const }; return action === 'delete' ? responseData(deleteProject({ ...request, body: { ...request.body, confirmation: confirmationValue } })) : responseData(restoreProject(request)) }, onMutate: () => { setConflict(false) }, onSuccess: (nextProject) => { setConflict(false); void client.setQueryData(['admin-project', project.id], nextProject); setConfirming(false) }, onError: (error) => { setConfirming(false); if (isProblem(error) && error.code === 'revision_conflict') { conflictRevision.current = project.revision; setConflict(true) } } })
  // A successful reload delivers a different revision; only then is the
  // recovery complete and the stale mutation error cleared. A failed reload
  // leaves the revision unchanged and keeps the recovery feedback visible.
  useEffect(() => {
    if (conflict && project.revision !== conflictRevision.current) {
      setConflict(false)
      setReloadFailed(false)
      mutation.reset()
    }
  }, [conflict, project.revision, mutation])
  // The reload fetch uses a distinct cache entry so a failed reload cannot
  // push the observed Project query into its error state and replace the form.
  const reloadRecord = async () => {
    setReloadFailed(false)
    setReloading(true)
    try {
      const fresh = await client.fetchQuery({
        queryKey: ['admin-project', project.id, 'reload'],
        queryFn: async () => (await getAdminProject({ path: { project_id: project.id }, throwOnError: true })).data,
        staleTime: 0,
      })
      void client.setQueryData(['admin-project', project.id], fresh)
    } catch {
      setReloadFailed(true)
    } finally {
      setReloading(false)
    }
  }
  const conflictNotice = conflict ? <div className={styles.conflict} role="alert"><p>{t('admin.conflict')}</p><button className={styles.secondaryButton} disabled={reloading} type="button" onClick={() => void reloadRecord()}>{t('action.reload')}</button>{reloading ? <p role="status">{t('feedback.working')}</p> : null}</div> : null
  const reloadFailureNotice = reloadFailed && !reloading ? <p className={styles.error} role="alert">{t('admin.projectReloadFailed')}</p> : null
  const failureNotice = mutation.isError && !conflict ? <p className={styles.error} role="alert">{t('admin.lifecycleFailed')}</p> : null
  const pendingNotice = mutation.isPending ? <p role="status">{t('feedback.working')}</p> : null
  if (project.status === 'deleted') return <div><button className={styles.secondaryButton} disabled={mutation.isPending || !csrfToken} type="button" onClick={() => mutation.mutate('restore')}>{t('action.restore')}</button>{pendingNotice}{conflictNotice}{reloadFailureNotice}{failureNotice}</div>
  return <div>{confirming ? <DeleteConfirmation confirmationValue={confirmationValue} onCancel={() => setConfirming(false)} onConfirm={() => mutation.mutate('delete')} pending={mutation.isPending} /> : null}<button className={styles.dangerButton} disabled={!confirmationValue || mutation.isPending} type="button" onClick={() => setConfirming(true)}>{t('action.delete')}</button>{pendingNotice}{conflictNotice}{reloadFailureNotice}{failureNotice}</div>
}

function AdminSearchPage() { const { t } = useTranslation(); const { csrfToken } = useSession(); return <div><PageTitle title={t('admin.searchTitle')} /><AdminSearchMaintenance csrfToken={csrfToken} /></div> }
function AdminImportUploadPage() { const { t } = useTranslation(); const { csrfToken } = useSession(); return <div><PageTitle title={t('imports.title')} /><AdminImportUpload csrfToken={csrfToken} /></div> }
function AdminImportReviewPage() { const { t } = useTranslation(); const { csrfToken } = useSession(); return <div><PageTitle title={t('imports.review')} /><AdminImportReview csrfToken={csrfToken} /></div> }
function AdminAuditPage() { const { t } = useTranslation(); return <div><PageTitle title={t('audit.title')} /><AdminAuditLog /></div> }
function AdminProjectsRoutePage() { const { t } = useTranslation(); return <div><PageTitle title={t('admin.projectsTitle')} /><AdminProjectList /></div> }
function AdminPeopleRoutePage() { const { t } = useTranslation(); const { csrfToken } = useSession(); return <div><PageTitle title={t('admin.peopleTitle')} /><AdminPeopleList csrfToken={csrfToken} /></div> }
function AdminPersonRoutePage() { const { t } = useTranslation(); const { csrfToken } = useSession(); return <div><PageTitle title={t('admin.personForm')} /><AdminPersonForm csrfToken={csrfToken} /></div> }
function AdminProjectNewPage() { const { t } = useTranslation(); return <div><PageTitle title={t('admin.newProject')} /><ProjectFormPage isNew /></div> }
function AdminProjectEditPage() { return <ProjectFormPage isNew={false} /> }

export function AppRoutes() { return <Routes><Route element={<AppFrame />}><Route index element={<HomePage />} /><Route path="search" element={<SearchPage />} /><Route path="projects/:projectId" element={<ProjectPage />} /><Route path="people/:personId" element={<PersonPage />} /><Route path="about" element={<AboutPage />} /><Route path="privacy" element={<InformationalPage bodyKey="privacyBody" titleKey="privacyTitle" />} /><Route path="accessibility" element={<InformationalPage bodyKey="accessibilityBody" titleKey="accessibilityTitle" />} /><Route path="terms" element={<InformationalPage bodyKey="termsBody" titleKey="termsTitle" />} /><Route path="contact" element={<InformationalPage bodyKey="contactBody" titleKey="contactTitle" />} /><Route path="admin/login" element={<LoginPage />} /><Route path="admin" element={<AdminGuard />}><Route index element={<AdminHome />} /><Route path="projects" element={<AdminProjectsRoutePage />} /><Route path="projects/new" element={<AdminProjectNewPage />} /><Route path="projects/:projectId/edit" element={<AdminProjectEditPage />} /><Route path="people" element={<AdminPeopleRoutePage />} /><Route path="people/:personId" element={<AdminPersonRoutePage />} /><Route path="imports" element={<AdminImportUploadPage />} /><Route path="imports/:batchId" element={<AdminImportReviewPage />} /><Route path="search" element={<AdminSearchPage />} /><Route path="audit" element={<AdminAuditPage />} /></Route><Route path="*" element={<NotFound />} /></Route></Routes> }

export function App() {
  useEffect(() => installSessionExpiryNotification(() => queryClient.setQueryData(['session'], null)), [])
  return <ApplicationErrorBoundary><I18nextProvider i18n={i18n}><BrowserRouter basename={routerBasename}><QueryClientProvider client={queryClient}><SessionProvider><AppRoutes /></SessionProvider></QueryClientProvider></BrowserRouter></I18nextProvider></ApplicationErrorBoundary>
}

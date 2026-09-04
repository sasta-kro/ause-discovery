import { useState } from 'react'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { listAdminProjects } from '../../api/generated/sdk.gen'
import type { ProjectStatus } from '../../api/generated/types.gen'
import styles from '../../App.module.css'
import { FormField } from './form-fields'
import { useCursorPage } from './paging'

type ProjectFilters = { query: string; status: '' | ProjectStatus }

const pageLimit = 20

export function AdminProjectList() {
  const { t } = useTranslation()
  const [filters, setFilters] = useState<ProjectFilters>({ query: '', status: '' })
  const [applied, setApplied] = useState<ProjectFilters>({ query: '', status: '' })
  const paging = useCursorPage()
  const projectsQuery = useQuery({
    queryKey: ['admin-projects', applied.query, applied.status, paging.cursor],
    queryFn: async () => (await listAdminProjects({ query: { q: applied.query || undefined, status: applied.status || undefined, cursor: paging.cursor ?? undefined, limit: pageLimit }, throwOnError: true })).data,
    placeholderData: keepPreviousData,
  })
  const pending = projectsQuery.isFetching
  const applyFilters = () => {
    paging.reset()
    setApplied({ ...filters })
  }
  const clearFilters = () => {
    setFilters({ query: '', status: '' })
    setApplied({ query: '', status: '' })
    paging.reset()
  }
  return <div>
    <div className={styles.resultsHeader}><h1>{t('admin.projectsTitle')}</h1><Link className={styles.button} to="new">{t('admin.newProject')}</Link></div>
    <form className={styles.form} onSubmit={(event) => { event.preventDefault(); applyFilters() }}>
      <FormField label={t('admin.projectFilter')}><input value={filters.query} onChange={(event) => setFilters({ ...filters, query: event.target.value })} /></FormField>
      <FormField label={t('fields.status')}><select value={filters.status} onChange={(event) => setFilters({ ...filters, status: event.target.value as ProjectFilters['status'] })}><option value="" /><option value="draft">{t('admin.draft')}</option><option value="published">{t('admin.published')}</option><option value="deleted">{t('admin.deleted')}</option></select></FormField>
      <div className={styles.formActions}><button className={styles.button} disabled={pending} type="submit">{t('action.filter')}</button><button className={styles.secondaryButton} disabled={pending} type="button" onClick={clearFilters}>{t('action.clear')}</button></div>
    </form>
    {projectsQuery.isPending ? <p role="status">{t('feedback.loading')}</p> : null}
    {projectsQuery.isError ? <p className={styles.error} role="alert">{t('admin.requestFailed')}</p> : null}
    {!projectsQuery.isPending && projectsQuery.data && projectsQuery.data.items.length === 0 ? <p>{t('admin.emptyProjects')}</p> : null}
    {projectsQuery.data && projectsQuery.data.items.length > 0 ? <div className={styles.tableWrap}><table className={styles.table}><thead><tr><th>{t('fields.title')}</th><th>{t('fields.status')}</th><th>{t('fields.year')}</th><th><span className="sr-only">{t('action.edit')}</span></th></tr></thead><tbody>{projectsQuery.data.items.map((project) => <tr key={project.id}><td>{project.title ?? t('admin.untitled')}</td><td>{t(`admin.${project.status}`)}</td><td>{project.academic_year}</td><td><Link to={`${project.id}/edit`}>{t('action.edit')}</Link></td></tr>)}</tbody></table></div> : null}
    <div className={styles.formActions}><button className={styles.secondaryButton} disabled={pending || !paging.cursorHistory.length} type="button" onClick={paging.previousPage}>{t('admin.previousPage')}</button><button className={styles.secondaryButton} disabled={pending || !projectsQuery.data?.page.next_cursor} type="button" onClick={() => paging.nextPage(projectsQuery.data?.page.next_cursor)}>{t('admin.nextPage')}</button></div>
  </div>
}

import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { createSearchRebuild, getSearchRebuild, getSearchStatus, reindexProject } from '../../api/generated/sdk.gen'
import styles from '../../App.module.css'

export function AdminSearchMaintenance({ csrfToken }: { csrfToken: string | null }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [projectID, setProjectID] = useState('')
  const [operationID, setOperationID] = useState('')

  const statusQuery = useQuery({
    queryKey: ['admin-search-status'],
    queryFn: async () => (await getSearchStatus({ throwOnError: true })).data,
    refetchInterval: (query) => query.state.data?.active_rebuild ? 2_000 : false,
  })
  const activeOperationID = operationID || statusQuery.data?.active_rebuild?.id || ''
  const rebuildQuery = useQuery({
    queryKey: ['admin-search-rebuild', activeOperationID],
    queryFn: async () => (await getSearchRebuild({ path: { operation_id: activeOperationID }, throwOnError: true })).data,
    enabled: activeOperationID !== '',
    refetchInterval: (query) => query.state.data && ['completed', 'failed'].includes(query.state.data.state) ? false : 2_000,
  })
  const reindexMutation = useMutation({
    mutationFn: async () => {
      if (!csrfToken) throw new Error('CSRF token unavailable')
      return (await reindexProject({ path: { project_id: projectID.trim() }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true })).data
    },
    onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['admin-search-status'] }) },
  })
  const rebuildMutation = useMutation({
    mutationFn: async () => {
      if (!csrfToken) throw new Error('CSRF token unavailable')
      return (await createSearchRebuild({ headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true })).data
    },
    onSuccess: (operation) => {
      setOperationID(operation.id)
      void queryClient.invalidateQueries({ queryKey: ['admin-search-status'] })
    },
  })
  const operation = rebuildQuery.data ?? rebuildMutation.data ?? statusQuery.data?.active_rebuild
  const failed = statusQuery.isError || reindexMutation.isError || rebuildMutation.isError || rebuildQuery.isError

  return <div><h1>{t('admin.searchTitle')}</h1>
    {statusQuery.isPending ? <p role="status">{t('feedback.loading')}</p> : null}
    {statusQuery.data ? <section className={styles.panel} aria-labelledby="search-status-title"><h2 id="search-status-title">{t('admin.searchStatus')}</h2><p className={statusQuery.data.available ? styles.notice : styles.error}>{t(statusQuery.data.available ? 'admin.searchAvailable' : 'admin.searchUnavailable')}</p><div className={styles.metadata}><span>{t('admin.searchPending', { count: statusQuery.data.pending_count })}</span><span>{t('admin.searchFailed', { count: statusQuery.data.failed_count })}</span></div></section> : null}
    <section className={styles.detailSection}><h2>{t('admin.projectReindex')}</h2><p>{t('admin.projectReindexHelp')}</p><form className={styles.form} onSubmit={(event) => { event.preventDefault(); reindexMutation.mutate() }}><FormField label={t('fields.projectId')}><input value={projectID} onChange={(event) => setProjectID(event.target.value)} required /></FormField><button className={styles.button} disabled={!csrfToken || !projectID.trim() || reindexMutation.isPending} type="submit">{t('action.reindexProject')}</button></form>{reindexMutation.data ? <p role="status">{t('admin.reindexQueued', { revision: reindexMutation.data.desired_revision })}</p> : null}</section>
    <section className={styles.detailSection}><h2>{t('admin.fullRebuild')}</h2><p>{t('admin.fullRebuildHelp')}</p><button className={styles.dangerButton} disabled={!csrfToken || rebuildMutation.isPending || operation?.state === 'pending' || operation?.state === 'processing'} type="button" onClick={() => rebuildMutation.mutate()}>{t('action.rebuildSearchIndex')}</button>{operation ? <div className={styles.panel} role="status"><strong>{t(`admin.rebuildState.${operation.state}`)}</strong><p>{t('admin.rebuildProgress', { processed: operation.processed_projects, total: operation.total_projects })}</p>{operation.error_code ? <p className={styles.error}>{operation.error_code}</p> : null}</div> : null}</section>
    {failed ? <p className={styles.error} role="alert">{t('feedback.searchMaintenanceFailed')}</p> : null}
  </div>
}

function FormField({ label, children }: { label: string; children: React.ReactNode }) {
  return <div className={styles.field}><label><span>{label}</span>{children}</label></div>
}

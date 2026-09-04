import { useState } from 'react'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { listAuditEvents } from '../../api/generated/sdk.gen'
import type { AuditEvent } from '../../api/generated/types.gen'
import styles from '../../App.module.css'

const actorUUIDPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/
const pageLimit = 20
const maximumMetadataText = 300

type AuditFilters = { action: string; actorId: string }

export function AdminAuditLog() {
  const { t } = useTranslation()
  const [filters, setFilters] = useState<AuditFilters>({ action: '', actorId: '' })
  const [applied, setApplied] = useState<AuditFilters>({ action: '', actorId: '' })
  const [invalidActor, setInvalidActor] = useState(false)
  const [cursor, setCursor] = useState<string | null>(null)
  const [cursorHistory, setCursorHistory] = useState<Array<string | null>>([])

  const auditQuery = useQuery({
    queryKey: ['admin-audit', applied.action, applied.actorId, cursor],
    queryFn: async () => (await listAuditEvents({ query: { action: applied.action || undefined, actor_id: applied.actorId || undefined, cursor: cursor ?? undefined, limit: pageLimit }, throwOnError: true })).data,
    placeholderData: keepPreviousData,
  })
  const pending = auditQuery.isFetching

  const applyFilters = () => {
    if (filters.actorId && !actorUUIDPattern.test(filters.actorId)) {
      setInvalidActor(true)
      return
    }
    setInvalidActor(false)
    setApplied({ ...filters })
    setCursor(null)
    setCursorHistory([])
  }
  const clearFilters = () => {
    setFilters({ action: '', actorId: '' })
    setApplied({ action: '', actorId: '' })
    setInvalidActor(false)
    setCursor(null)
    setCursorHistory([])
  }
  const nextPage = () => {
    const nextCursor = auditQuery.data?.page.next_cursor
    if (!nextCursor) return
    setCursorHistory((previous) => [...previous, cursor])
    setCursor(nextCursor)
  }
  const previousPage = () => {
    if (!cursorHistory.length) return
    setCursor(cursorHistory[cursorHistory.length - 1])
    setCursorHistory((previous) => previous.slice(0, -1))
  }

  return <div><h1>{t('audit.title')}</h1><p className={styles.lede}>{t('audit.description')}</p>
    <form className={styles.form} onSubmit={(event) => { event.preventDefault(); applyFilters() }}>
      <FormField label={t('audit.actionFilter')}><input value={filters.action} onChange={(event) => setFilters({ ...filters, action: event.target.value })} /></FormField>
      <FormField label={t('audit.actorFilter')}><input value={filters.actorId} onChange={(event) => setFilters({ ...filters, actorId: event.target.value })} inputMode="text" spellCheck={false} /></FormField>
      <div className={styles.formActions}><button className={styles.button} disabled={pending} type="submit">{t('action.filter')}</button><button className={styles.secondaryButton} disabled={pending} type="button" onClick={clearFilters}>{t('action.clear')}</button></div>
    </form>
    {invalidActor ? <p className={styles.error} role="alert">{t('audit.invalidActor')}</p> : null}
    {auditQuery.isError ? <p className={styles.error} role="alert">{t('audit.failed')}</p> : null}
    {auditQuery.isPending ? <p role="status">{t('feedback.loading')}</p> : null}
    {auditQuery.data ? (auditQuery.data.items.length === 0 ? <p>{t('audit.empty')}</p> : <div className={styles.tableWrap}><table className={styles.table}><thead><tr><th>{t('audit.timestamp')}</th><th>{t('audit.action')}</th><th>{t('audit.actor')}</th><th>{t('audit.resourceType')}</th><th>{t('audit.resourceId')}</th><th>{t('audit.metadata')}</th></tr></thead><tbody>{auditQuery.data.items.map((event) => <AuditRow event={event} key={event.id} />)}</tbody></table></div>) : null}
    <div className={styles.formActions}><button className={styles.secondaryButton} disabled={pending || !cursorHistory.length} type="button" onClick={previousPage}>{t('audit.previous')}</button><button className={styles.secondaryButton} disabled={pending || !auditQuery.data?.page.next_cursor} type="button" onClick={nextPage}>{t('audit.next')}</button></div>
  </div>
}

function AuditRow({ event }: { event: AuditEvent }) {
  const { t } = useTranslation()
  return <tr>
    <td>{formatTimestamp(event.created_at)}</td>
    <td>{event.action}</td>
    <td>{event.actor_id ?? <span className={styles.metadata}>{t('audit.systemActor')}</span>}</td>
    <td>{event.resource_type}</td>
    <td>{event.resource_id ?? '-'}</td>
    <td><MetadataDetails metadata={event.metadata} /></td>
  </tr>
}

function MetadataDetails({ metadata }: { metadata: Record<string, unknown> }) {
  const { t } = useTranslation()
  const entries = Object.entries(metadata)
  if (!entries.length) return <span className={styles.metadata}>{t('audit.noMetadata')}</span>
  return <dl className={styles.definitionList}>{entries.map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{formatMetadataValue(value)}</dd></div>)}</dl>
}

function formatMetadataValue(value: unknown): string {
  if (value === null) return 'null'
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') return String(value)
  const text = JSON.stringify(value) ?? 'null'
  return text.length > maximumMetadataText ? text.slice(0, maximumMetadataText) : text
}

function formatTimestamp(value: string): string {
  return new Date(value).toISOString().replace('T', ' ').replace('Z', ' UTC')
}

function FormField({ label, children }: { label: string; children: React.ReactNode }) {
  return <div className={styles.field}><label><span>{label}</span>{children}</label></div>
}

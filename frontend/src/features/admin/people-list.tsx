import { useState } from 'react'
import { keepPreviousData, useMutation, useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { useForm } from 'react-hook-form'
import { createPerson, listAdminPeople } from '../../api/generated/sdk.gen'
import { isProblem } from '../../app/problem'
import styles from '../../App.module.css'
import { FormField } from './form-fields'
import { useCursorPage } from './paging'

const pageLimit = 20
const studentIDPattern = /^\d{7}$/

type PersonValues = { displayName: string; studentId: string; staffId: string }

export function AdminPeopleList({ csrfToken }: { csrfToken: string | null }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [filter, setFilter] = useState('')
  const [applied, setApplied] = useState('')
  const [serverIssues, setServerIssues] = useState<Array<{ field: string; message?: string }>>([])
  const [generalFailure, setGeneralFailure] = useState(false)
  const paging = useCursorPage()
  const peopleQuery = useQuery({
    queryKey: ['admin-people', applied, paging.cursor],
    queryFn: async () => (await listAdminPeople({ query: { q: applied || undefined, cursor: paging.cursor ?? undefined, limit: pageLimit }, throwOnError: true })).data,
    placeholderData: keepPreviousData,
  })
  const form = useForm<PersonValues>({ defaultValues: { displayName: '', studentId: '', staffId: '' } })
  const createMutation = useMutation({
    mutationFn: async (values: PersonValues) => {
      if (!csrfToken) throw new Error('CSRF token unavailable')
      return (await createPerson({ body: { display_name: values.displayName, student_id: values.studentId || null, staff_id: values.staffId || null }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true })).data
    },
    onSuccess: (person) => { setServerIssues([]); setGeneralFailure(false); navigate(`/admin/people/${person.id}`) },
    onError: (error) => {
      if (isProblem(error) && 'issues' in error) { setServerIssues((error as { issues: Array<{ field: string; message?: string }> }).issues); return }
      setGeneralFailure(true)
    },
  })
  const pending = peopleQuery.isFetching
  return <div>
    <h1>{t('admin.peopleTitle')}</h1>
    <section className={styles.panel}><h2>{t('admin.newPerson')}</h2>
      <form className={styles.form} onSubmit={form.handleSubmit((values) => createMutation.mutate(values))}>
        <FormField label={t('fields.displayName')} error={form.formState.errors.displayName?.message}><input {...form.register('displayName', { required: t('feedback.required') })} /></FormField>
        <FormField label={t('fields.studentId')} error={form.formState.errors.studentId?.message}><input {...form.register('studentId', { validate: (value) => !value || studentIDPattern.test(value) || t('feedback.invalidStudentId') })} inputMode="numeric" /></FormField>
        <FormField label={t('fields.staffId')}><input {...form.register('staffId')} /></FormField>
        <button className={styles.button} disabled={createMutation.isPending || !csrfToken} type="submit">{t('action.create')}</button>
      </form>
      {serverIssues.length ? <div className={styles.error} role="alert"><strong>{t('admin.serverIssues')}</strong><ul>{serverIssues.map((issue, index) => <li key={`${issue.field}-${index}`}>{issue.message ?? issue.field}</li>)}</ul></div> : null}
      {generalFailure ? <p className={styles.error} role="alert">{t('admin.personSaveFailed')}</p> : null}
    </section>
    <form className={styles.form} onSubmit={(event) => { event.preventDefault(); paging.reset(); setApplied(filter) }}>
      <FormField label={t('admin.personFilter')}><input value={filter} onChange={(event) => setFilter(event.target.value)} /></FormField>
      <div className={styles.formActions}><button className={styles.button} disabled={pending} type="submit">{t('action.filter')}</button><button className={styles.secondaryButton} disabled={pending} type="button" onClick={() => { setFilter(''); setApplied(''); paging.reset() }}>{t('action.clear')}</button></div>
    </form>
    {peopleQuery.isPending ? <p role="status">{t('feedback.loading')}</p> : null}
    {peopleQuery.isError ? <p className={styles.error} role="alert">{t('admin.requestFailed')}</p> : null}
    {!peopleQuery.isPending && peopleQuery.data && peopleQuery.data.items.length === 0 ? <p>{t('admin.emptyPeople')}</p> : null}
    {peopleQuery.data && peopleQuery.data.items.length > 0 ? <div className={styles.tableWrap}><table className={styles.table}><thead><tr><th>{t('fields.displayName')}</th><th>{t('fields.studentId')}</th><th>{t('fields.staffId')}</th><th><span className="sr-only">{t('action.edit')}</span></th></tr></thead><tbody>{peopleQuery.data.items.map((person) => <tr key={person.id}><td>{person.display_name}</td><td>{person.student_id}</td><td>{person.staff_id}</td><td><Link to={person.id}>{t('action.edit')}</Link></td></tr>)}</tbody></table></div> : null}
    <div className={styles.formActions}><button className={styles.secondaryButton} disabled={pending || !paging.cursorHistory.length} type="button" onClick={paging.previousPage}>{t('admin.previousPage')}</button><button className={styles.secondaryButton} disabled={pending || !peopleQuery.data?.page.next_cursor} type="button" onClick={() => paging.nextPage(peopleQuery.data?.page.next_cursor)}>{t('admin.nextPage')}</button></div>
  </div>
}

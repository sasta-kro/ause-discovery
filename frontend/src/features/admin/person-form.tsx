import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Link, useParams } from 'react-router'
import { useForm } from 'react-hook-form'
import { getAdminPerson, updatePerson } from '../../api/generated/sdk.gen'
import { isProblem } from '../../app/problem'
import styles from '../../App.module.css'
import { FormField } from './form-fields'

const studentIDPattern = /^\d{7}$/

type PersonValues = { displayName: string; studentId: string; staffId: string }

function toValues(person?: { display_name: string; student_id?: string | null; staff_id?: string | null }): PersonValues {
  return { displayName: person?.display_name ?? '', studentId: person?.student_id ?? '', staffId: person?.staff_id ?? '' }
}

export function AdminPersonForm({ csrfToken }: { csrfToken: string | null }) {
  const { t } = useTranslation()
  const { personId = '' } = useParams()
  const client = useQueryClient()
  const personQuery = useQuery({ queryKey: ['admin-person', personId], queryFn: async () => (await getAdminPerson({ path: { person_id: personId }, throwOnError: true })).data })
  const [conflict, setConflict] = useState(false)
  const [serverIssues, setServerIssues] = useState<Array<{ field: string; message?: string }>>([])
  const [generalFailure, setGeneralFailure] = useState(false)
  const [reloading, setReloading] = useState(false)
  const [reloadFailed, setReloadFailed] = useState(false)
  const form = useForm<PersonValues>({ defaultValues: toValues() })
  const loadedRecordID = useRef<string | null>(null)
  const activePersonID = useRef(personId)
  const reloadOwner = useRef(0)

  // A reload adopts only the actual server response, never cached data, so the
  // form and the revision used for the next save always describe one record.
  // The fetch uses a distinct cache entry so a failed reload cannot push the
  // page-level query into its error state. Each request owns the shared
  // loading flag by token, and a response that arrives after navigation to
  // another Person is applied to the cache only.
  const reloadRecord = async () => {
    const requestedID = personId
    const owner = reloadOwner.current + 1
    reloadOwner.current = owner
    setReloadFailed(false)
    setReloading(true)
    try {
      const fresh = await client.fetchQuery({
        queryKey: ['admin-person', requestedID, 'reload'],
        queryFn: async () => (await getAdminPerson({ path: { person_id: requestedID }, throwOnError: true })).data,
        staleTime: 0,
      })
      void client.setQueryData(['admin-person', requestedID], fresh)
      if (owner !== reloadOwner.current) return
      if (requestedID !== activePersonID.current) return
      loadedRecordID.current = fresh.id
      form.reset(toValues(fresh))
      setConflict(false)
    } catch {
      if (owner === reloadOwner.current && requestedID === activePersonID.current) setReloadFailed(true)
    } finally {
      if (owner === reloadOwner.current) setReloading(false)
    }
  }

  const mutation = useMutation({
    mutationFn: async (values: PersonValues) => {
      if (!csrfToken || !personQuery.data) throw new Error('CSRF token unavailable')
      return (await updatePerson({ path: { person_id: personId }, body: { expected_revision: personQuery.data.revision, display_name: values.displayName, student_id: values.studentId || null, staff_id: values.staffId || null }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true })).data
    },
    onSuccess: (person) => {
      setConflict(false); setServerIssues([]); setGeneralFailure(false); setReloadFailed(false)
      void client.setQueryData(['admin-person', person.id], person)
    },
    onError: (error) => {
      if (isProblem(error) && error.code === 'revision_conflict') { setConflict(true); return }
      if (isProblem(error) && 'issues' in error) { setServerIssues((error as { issues: Array<{ field: string; message?: string }> }).issues); return }
      setGeneralFailure(true)
    },
  })

  // Navigation between Person routes reuses this component, so record changes
  // adopt the new record's fields, revision, and clean feedback state.
  useEffect(() => {
    const person = personQuery.data
    if (!person || loadedRecordID.current === person.id) return
    loadedRecordID.current = person.id
    activePersonID.current = person.id
    form.reset(toValues(person))
    setConflict(false)
    setServerIssues([])
    setGeneralFailure(false)
    setReloadFailed(false)
    setReloading(false)
    mutation.reset()
  }, [form, personQuery.data, mutation])

  if (personQuery.isPending) return <p role="status">{t('feedback.loading')}</p>
  if (personQuery.isError) {
    const notFound = isProblem(personQuery.error) && personQuery.error.code === 'not_found'
    return <div><h1>{t('admin.personForm')}</h1><p className={styles.error} role="alert">{notFound ? t('feedback.recordNotFound') : t('admin.requestFailed')}</p><p className={styles.formActions}><Link className={styles.secondaryButton} to="/admin/people">{t('admin.peopleTitle')}</Link></p></div>
  }
  return <div><h1>{t('admin.personForm')}</h1>
    {conflict ? <div className={styles.conflict} role="alert"><p>{t('admin.conflict')}</p><button className={styles.secondaryButton} disabled={reloading} onClick={() => void reloadRecord()} type="button">{t('action.reload')}</button>{reloading ? <p role="status">{t('feedback.working')}</p> : null}</div> : null}
    {reloadFailed ? <p className={styles.error} role="alert">{t('admin.personReloadFailed')}</p> : null}
    {serverIssues.length ? <div className={styles.error} role="alert"><strong>{t('admin.serverIssues')}</strong><ul>{serverIssues.map((issue, index) => <li key={`${issue.field}-${index}`}>{issue.message ?? issue.field}</li>)}</ul></div> : null}
    {generalFailure ? <p className={styles.error} role="alert">{t('admin.personSaveFailed')}</p> : null}
    {mutation.isSuccess && !mutation.isPending ? <p role="status">{t('feedback.saved')}</p> : null}
    <form className={styles.form} onSubmit={form.handleSubmit((values) => mutation.mutate(values))}>
      <FormField label={t('fields.displayName')} error={form.formState.errors.displayName?.message}><input {...form.register('displayName', { required: t('feedback.required') })} /></FormField>
      <FormField label={t('fields.studentId')} error={form.formState.errors.studentId?.message}><input {...form.register('studentId', { validate: (value) => !value || studentIDPattern.test(value) || t('feedback.invalidStudentId') })} inputMode="numeric" /></FormField>
      <FormField label={t('fields.staffId')}><input {...form.register('staffId')} /></FormField>
      <button className={styles.button} disabled={mutation.isPending || reloading || !csrfToken} type="submit">{t('action.saveDraft')}</button>
    </form>
  </div>
}

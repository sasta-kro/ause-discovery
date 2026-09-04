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
  const form = useForm<PersonValues>({ defaultValues: toValues() })
  const loadedID = useRef<string | null>(null)
  const [reloadRequested, setReloadRequested] = useState(false)

  useEffect(() => {
    const person = personQuery.data
    if (!person) return
    if (loadedID.current !== person.id || reloadRequested) {
      loadedID.current = person.id
      setReloadRequested(false)
      form.reset(toValues(person))
    }
  }, [form, personQuery.data, reloadRequested])

  const mutation = useMutation({
    mutationFn: async (values: PersonValues) => {
      if (!csrfToken || !personQuery.data) throw new Error('CSRF token unavailable')
      return (await updatePerson({ path: { person_id: personId }, body: { expected_revision: personQuery.data.revision, display_name: values.displayName, student_id: values.studentId || null, staff_id: values.staffId || null }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true })).data
    },
    onSuccess: (person) => {
      setConflict(false); setServerIssues([]); setGeneralFailure(false)
      void client.setQueryData(['admin-person', person.id], person)
    },
    onError: (error) => {
      if (isProblem(error) && error.code === 'revision_conflict') { setConflict(true); return }
      if (isProblem(error) && 'issues' in error) { setServerIssues((error as { issues: Array<{ field: string; message?: string }> }).issues); return }
      setGeneralFailure(true)
    },
  })

  if (personQuery.isPending) return <p role="status">{t('feedback.loading')}</p>
  if (personQuery.isError) {
    const notFound = isProblem(personQuery.error) && personQuery.error.code === 'not_found'
    return <div><h1>{t('admin.personForm')}</h1><p className={styles.error} role="alert">{notFound ? t('feedback.recordNotFound') : t('admin.requestFailed')}</p><p className={styles.formActions}><Link className={styles.secondaryButton} to="/admin/people">{t('admin.peopleTitle')}</Link></p></div>
  }
  return <div><h1>{t('admin.personForm')}</h1>
    {conflict ? <div className={styles.conflict} role="alert"><p>{t('admin.conflict')}</p><button className={styles.secondaryButton} onClick={() => { setConflict(false); setReloadRequested(true); void client.invalidateQueries({ queryKey: ['admin-person', personId] }) }}>{t('action.reload')}</button></div> : null}
    {serverIssues.length ? <div className={styles.error} role="alert"><strong>{t('admin.serverIssues')}</strong><ul>{serverIssues.map((issue, index) => <li key={`${issue.field}-${index}`}>{issue.message ?? issue.field}</li>)}</ul></div> : null}
    {generalFailure ? <p className={styles.error} role="alert">{t('admin.personSaveFailed')}</p> : null}
    {mutation.isSuccess && !mutation.isPending ? <p role="status">{t('feedback.saved')}</p> : null}
    <form className={styles.form} onSubmit={form.handleSubmit((values) => mutation.mutate(values))}>
      <FormField label={t('fields.displayName')} error={form.formState.errors.displayName?.message}><input {...form.register('displayName', { required: t('feedback.required') })} /></FormField>
      <FormField label={t('fields.studentId')} error={form.formState.errors.studentId?.message}><input {...form.register('studentId', { validate: (value) => !value || studentIDPattern.test(value) || t('feedback.invalidStudentId') })} inputMode="numeric" /></FormField>
      <FormField label={t('fields.staffId')}><input {...form.register('staffId')} /></FormField>
      <button className={styles.button} disabled={mutation.isPending || !csrfToken} type="submit">{t('action.saveDraft')}</button>
    </form>
  </div>
}

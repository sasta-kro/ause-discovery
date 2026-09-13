import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { removeProjectLogo, uploadProjectLogo } from '../../api/generated/sdk.gen'
import type { AdminProject } from '../../api/generated/types.gen'
import { isProblem } from '../../app/problem'
import { projectInitials } from '../search/filter-controls'
import { useDecodedLogo } from '../../decoded-logo'
import styles from '../../App.module.css'

const maxLogoBytes = 2 * 1024 * 1024

type LogoValues = { file: FileList }

async function responseOf<T>(request: Promise<{ data: T }>): Promise<T> {
  return (await request).data
}

export function ProjectLogoManagement({ project, csrfToken, disabled = false }: { project: AdminProject; csrfToken: string | null; disabled?: boolean }) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const [conflict, setConflict] = useState(false)
  const [invalidFile, setInvalidFile] = useState<string | null>(null)
  const { status: previewStatus, imageProps: previewImageProps } = useDecodedLogo(project.logo_url)
  const previewUrl = project.logo_url
  const previewReady = previewStatus === 'ready'
  const showPreview = previewUrl && previewStatus !== 'failed' && previewStatus !== 'absent'
  const form = useForm<LogoValues>()
  const refreshProject = async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: ['admin-project', project.id] }),
      client.invalidateQueries({ queryKey: ['admin-projects'] }),
      client.invalidateQueries({ queryKey: ['project', project.id] }),
    ])
  }
  const uploadMutation = useMutation({
    mutationFn: async (values: LogoValues) => {
      const file = values.file?.[0]
      if (!csrfToken || !file) throw new Error('Project Logo upload prerequisites are unavailable')
      return responseOf(uploadProjectLogo({ path: { project_id: project.id }, body: { expected_project_revision: project.revision, file }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true }))
    },
    onMutate: () => { setConflict(false); setInvalidFile(null) },
    onSuccess: async () => { form.reset(); await refreshProject() },
    onError: (error) => {
      if (isProblem(error) && error.code === 'revision_conflict') { setConflict(true); return }
      if (isProblem(error) && error.code === 'validation_error') { setInvalidFile(error.detail ?? t('admin.projectLogoInvalid')); return }
      setInvalidFile(t('admin.projectLogoInvalid'))
    },
  })
  const removeMutation = useMutation({
    mutationFn: async () => {
      if (!csrfToken) throw new Error('CSRF token unavailable')
      return responseOf(removeProjectLogo({ path: { project_id: project.id }, body: { expected_revision: project.revision }, headers: { 'X-CSRF-Token': csrfToken }, throwOnError: true }))
    },
    onMutate: () => { setConflict(false); setInvalidFile(null) },
    onSuccess: refreshProject,
    onError: (error) => {
      if (isProblem(error) && error.code === 'revision_conflict') setConflict(true)
    },
  })
  const pending = uploadMutation.isPending || removeMutation.isPending
  const failed = uploadMutation.isError || removeMutation.isError
  return <section className={styles.detailSection}>
    <h2>{t('admin.projectLogo')}</h2>
    <p>{t('admin.projectLogoHelp')}</p>
    <div className={styles.logoManagement}>
      <div aria-hidden="true" className={`${styles.pageHeaderIdentity} ${previewReady ? styles.pageHeaderIdentityImage : styles.pageHeaderIdentityPurple}`}>
        <span className={`${styles.pageHeaderInitials} ${previewReady ? styles.initialsConcealed : ''}`}>{projectInitials(project.title ?? '')}</span>
        {showPreview ? <img alt="" className={`${styles.pageHeaderLogo} ${previewReady ? styles.logoRevealed : styles.logoConcealed}`} decoding="async" loading="eager" src={previewUrl ?? undefined} {...previewImageProps} /> : null}
      </div>
      {disabled ? <p className={styles.notice}>{t('admin.projectLogoDeleted')}</p> : <>
        <form className={styles.artifactUpload} onSubmit={form.handleSubmit((values) => {
          const file = values.file?.[0]
          if (file && (file.type !== 'image/png' || file.size > maxLogoBytes)) { setInvalidFile(t('admin.projectLogoInvalid')); return }
          uploadMutation.mutate(values)
        })}>
          <label className={styles.field}>
            {t('fields.logoFile')}
            <input accept="image/png" type="file" {...form.register('file', { required: t('feedback.required') })} />
          </label>
          <button className={styles.secondaryButton} disabled={pending || !csrfToken} type="submit">{project.logo_url ? t('admin.projectLogoReplace') : t('admin.projectLogoUpload')}</button>
        </form>
        {project.logo_url ? <button className={styles.secondaryButton} disabled={pending || !csrfToken} type="button" onClick={() => removeMutation.mutate()}>{t('admin.projectLogoRemove')}</button> : null}
      </>}
    </div>
    {invalidFile ? <p className={styles.error} role="alert">{invalidFile}</p> : null}
    {conflict ? <p className={styles.conflict} role="alert">{t('admin.conflict')}</p> : null}
    {failed && !conflict && !invalidFile ? <p className={styles.error} role="alert">{t('admin.projectLogoFailed')}</p> : null}
    {pending ? <p role="status">{t('feedback.working')}</p> : null}
    {uploadMutation.isSuccess && !uploadMutation.isPending ? <p role="status">{t('feedback.saved')}</p> : null}
  </section>
}

import type { ReactNode } from 'react'
import styles from '../../App.module.css'

export function FormField({ label, error, children }: { label: string; error?: string; children: ReactNode }) {
  return <div className={styles.field}><label><span>{label}</span>{children}</label>{error ? <span className={styles.fieldError} role="alert">{error}</span> : null}</div>
}

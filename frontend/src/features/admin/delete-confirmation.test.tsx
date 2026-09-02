// @vitest-environment jsdom
import { fireEvent, render, screen } from '@testing-library/react'
import { I18nextProvider } from 'react-i18next'
import { describe, expect, it, vi } from 'vitest'
import i18n from '../../app/i18n'
import { DeleteConfirmation } from '../../app-shell'

describe('delete confirmation accessibility', () => {
  it('exposes a labelled modal dialog and requires the confirmation phrase', () => {
    const onConfirm = vi.fn()
    render(<I18nextProvider i18n={i18n}><DeleteConfirmation confirmationValue="REF-001" onCancel={vi.fn()} onConfirm={onConfirm} /></I18nextProvider>)
    expect(screen.getByRole('dialog', { name: 'Delete project' })).toBeTruthy()
    const deleteButton = screen.getByRole('button', { name: 'Delete' }) as HTMLButtonElement
    expect(deleteButton.disabled).toBe(true)
    fireEvent.change(screen.getByLabelText('Confirmation value'), { target: { value: 'REF-001' } })
    expect(deleteButton.disabled).toBe(false)
    fireEvent.click(deleteButton)
    expect(onConfirm).toHaveBeenCalledOnce()
  })
})

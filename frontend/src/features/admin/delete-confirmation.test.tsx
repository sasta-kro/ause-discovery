// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { I18nextProvider } from 'react-i18next'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import i18n from '../../app/i18n'
import { DeleteConfirmation } from '../../app-shell'

function Harness({ pending = false, onCancel, onConfirm }: { pending?: boolean; onCancel: () => void; onConfirm: () => void }) {
  const [open, setOpen] = useState(false)
  return (
    <div>
      <button onClick={() => setOpen(true)}>Trigger deletion</button>
      {open ? <DeleteConfirmation confirmationValue="REF-001" onCancel={() => { setOpen(false); onCancel() }} onConfirm={onConfirm} pending={pending} /> : null}
    </div>
  )
}

describe('delete confirmation accessibility', () => {
  afterEach(cleanup)

  it('exposes a labelled modal dialog and requires the confirmation phrase', () => {
    const onConfirm = vi.fn()
    render(<I18nextProvider i18n={i18n}><DeleteConfirmation confirmationValue="REF-001" onCancel={vi.fn()} onConfirm={onConfirm} /></I18nextProvider>)
    const dialog = screen.getByRole('dialog', { name: 'Delete project' })
    expect(dialog.getAttribute('aria-describedby')).toBe('delete-description')
    const deleteButton = screen.getByRole('button', { name: 'Delete' }) as HTMLButtonElement
    expect(deleteButton.disabled).toBe(true)
    fireEvent.change(screen.getByLabelText('Confirmation value'), { target: { value: 'REF-001' } })
    expect(deleteButton.disabled).toBe(false)
    fireEvent.click(deleteButton)
    expect(onConfirm).toHaveBeenCalledOnce()
  })

  it('focuses the confirmation input first, keeps focus inside, closes on Escape, and restores the trigger', async () => {
    const user = userEvent.setup()
    const onCancel = vi.fn()
    render(<Harness onCancel={onCancel} onConfirm={vi.fn()} />)

    const trigger = screen.getByRole('button', { name: 'Trigger deletion' })
    await user.click(trigger)

    const input = screen.getByLabelText('Confirmation value')
    expect(document.activeElement).toBe(input)

    await user.tab()
    const focusedButton = document.activeElement
    expect(['Delete', 'Cancel'].includes(focusedButton?.textContent ?? '')).toBe(true)

    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onCancel).toHaveBeenCalledOnce()
    expect(screen.queryByRole('dialog')).toBeNull()
    await Promise.resolve()
    expect(document.activeElement).toBe(trigger)
  })

  it('disables actions while the deletion is pending', () => {
    render(<I18nextProvider i18n={i18n}><DeleteConfirmation confirmationValue="REF-001" onCancel={vi.fn()} onConfirm={vi.fn()} pending /></I18nextProvider>)
    expect((screen.getByRole('button', { name: 'Delete' }) as HTMLButtonElement).disabled).toBe(true)
    expect((screen.getByRole('button', { name: 'Cancel' }) as HTMLButtonElement).disabled).toBe(true)
  })
})

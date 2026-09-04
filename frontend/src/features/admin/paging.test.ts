// @vitest-environment jsdom
import { act, renderHook } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { useCursorPage } from './paging'

describe('cursor paging state', () => {
  it('moves forward, keeps local history, and returns without synthesizing cursors', () => {
    const page = renderHook(() => useCursorPage())
    expect(page.result.current.cursor).toBeNull()
    expect(page.result.current.cursorHistory).toEqual([])

    act(() => page.result.current.nextPage('cursor-2'))
    expect(page.result.current.cursor).toBe('cursor-2')
    expect(page.result.current.cursorHistory).toEqual([null])

    act(() => page.result.current.nextPage('cursor-3'))
    expect(page.result.current.cursor).toBe('cursor-3')
    expect(page.result.current.cursorHistory).toEqual([null, 'cursor-2'])

    act(() => page.result.current.previousPage())
    expect(page.result.current.cursor).toBe('cursor-2')
    expect(page.result.current.cursorHistory).toEqual([null])

    act(() => page.result.current.previousPage())
    expect(page.result.current.cursor).toBeNull()
    expect(page.result.current.cursorHistory).toEqual([])

    act(() => page.result.current.previousPage())
    expect(page.result.current.cursor).toBeNull()
  })

  it('ignores absent next cursors and resets cleanly', () => {
    const page = renderHook(() => useCursorPage())
    act(() => page.result.current.nextPage(undefined))
    act(() => page.result.current.nextPage(null))
    expect(page.result.current.cursor).toBeNull()
    expect(page.result.current.cursorHistory).toEqual([])

    act(() => page.result.current.nextPage('cursor-2'))
    act(() => page.result.current.reset())
    expect(page.result.current.cursor).toBeNull()
    expect(page.result.current.cursorHistory).toEqual([])
  })
})

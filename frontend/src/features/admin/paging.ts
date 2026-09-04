import { useCallback, useState } from 'react'

/**
 * Cursor paging state for list pages. Page one has a null cursor; moving Next
 * stores the current cursor so Previous can return without synthesizing
 * reverse cursors. Filter changes call reset so stale positions are dropped.
 */
export function useCursorPage() {
  const [cursor, setCursor] = useState<string | null>(null)
  const [cursorHistory, setCursorHistory] = useState<Array<string | null>>([])

  const nextPage = useCallback((nextCursor: string | null | undefined) => {
    if (!nextCursor) return
    setCursorHistory((previous) => [...previous, cursor])
    setCursor(nextCursor)
  }, [cursor])

  const previousPage = useCallback(() => {
    setCursorHistory((previous) => {
      if (!previous.length) return previous
      setCursor(previous[previous.length - 1])
      return previous.slice(0, -1)
    })
  }, [])

  const reset = useCallback(() => {
    setCursor(null)
    setCursorHistory([])
  }, [])

  return { cursor, cursorHistory, nextPage, previousPage, reset }
}

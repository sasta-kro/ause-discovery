// @vitest-environment jsdom
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { highlightText } from './highlight'

describe('safe highlighting', () => {
  it('renders matching text without interpreting search content as markup', () => {
    render(<p>{highlightText('<script>alert(1)</script> archive', ['<script>'])}</p>)
    expect(screen.getByText('<script>')).toBeTruthy()
    expect(document.querySelector('script')).toBeNull()
  })
})

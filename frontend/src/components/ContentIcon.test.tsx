// @vitest-environment jsdom
import { cleanup, render } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { ContentIcon } from './ContentIcon.tsx'

afterEach(cleanup)

// lucide marks each drawing with a class named after the icon, e.g. "lucide-swords".
function drawn(name: string | null | undefined) {
  const { container } = render(<ContentIcon name={name} />)
  const svg = container.querySelector('svg')
  return svg?.getAttribute('class') ?? ''
}

describe('ContentIcon', () => {
  it('draws the icon named by the key from the database', () => {
    expect(drawn('swords')).toContain('lucide-swords')
    expect(drawn('car-front')).toContain('lucide-car-front')
  })

  it('draws the default icon for an unknown, empty or missing key', () => {
    for (const key of ['no-such-icon', '', null, undefined]) {
      expect(drawn(key)).toContain('lucide-gamepad')
    }
  })

  it('is hidden from screen readers (the text next to it says what it is)', () => {
    const { container } = render(<ContentIcon name="swords" />)
    expect(container.querySelector('svg')?.getAttribute('aria-hidden')).toBe('true')
  })
})

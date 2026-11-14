import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { spineAriaLabel } from '../lib/spineLabel'
import { Spine } from './Spine'

const colors = { color: '#2E2A22', accent: '#D7B55B', ink: '#FFFFFF' }

// Testy działają w środowisku node (bez DOM), więc czytamy wyrenderowany HTML.
function render(props: Partial<Parameters<typeof Spine>[0]> = {}) {
  const html = renderToStaticMarkup(
    <Spine title="Elden Ring" status="completed" platform="PS5" score={94} colors={colors} {...props} />,
  )
  const attr = (name: string) => html.match(new RegExp(`${name}="([^"]*)"`))?.[1] ?? null
  const style = attr('style') ?? ''
  return {
    getAttribute: attr,
    style: {
      getPropertyValue: (prop: string) => style.match(new RegExp(`${prop}:([^;]*)`))?.[1]?.trim() ?? '',
    },
    textContent: html.replace(/<[^>]+>/g, ''),
  }
}

describe('Spine', () => {
  it('ma aria-label z tytułem, statusem i oceną', () => {
    expect(render().getAttribute('aria-label')).toBe('Elden Ring, Ukończone, ocena 94')
  })

  it('pomija ocenę, gdy jej nie ma', () => {
    expect(render({ status: 'in_progress', score: null }).getAttribute('aria-label')).toBe('Elden Ring, W trakcie')
    expect(spineAriaLabel('Hades II', 'planned')).toBe('Hades II, Planowane')
  })

  it.each([
    ['PS5', '46px', '200px', 'PS5'],
    ['PS4', '46px', '200px', 'PS4'],
    ['Xbox Series', '46px', '200px', 'XBOX'],
    ['PC', '46px', '186px', 'PC'],
    ['Switch', '40px', '166px', 'NSW'],
  ])('platforma %s: %s × %s, skrót %s', (platform, w, h, short) => {
    const el = render({ platform })
    expect(el.style.getPropertyValue('--spine-w')).toBe(w)
    expect(el.style.getPropertyValue('--spine-h')).toBe(h)
    expect(el.textContent).toContain(short)
  })

  it('zaznaczenie przez aria-pressed', () => {
    expect(render().getAttribute('aria-pressed')).toBe('false')
    expect(render({ selected: true }).getAttribute('aria-pressed')).toBe('true')
  })

  it('kolory grzbietu i statusu idą do zmiennych CSS', () => {
    const el = render()
    expect(el.style.getPropertyValue('--spine-color')).toBe('#2E2A22')
    expect(el.style.getPropertyValue('--spine-ink')).toBe('#FFFFFF')
    expect(el.style.getPropertyValue('--spine-band')).toBe('var(--st-completed)')
  })
})

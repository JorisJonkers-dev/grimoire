import postcss from 'postcss'
import { describe, expect, it } from 'vitest'
import { accessibleCss } from './css'

const run = (css: string) => postcss([accessibleCss()]).process(css, { from: undefined }).css.replace(/\s+/g, ' ').trim()

describe('accessible css', () => {
  it('scales every text size by the chosen text size', () => {
    expect(run('.a { font-size: 14px; color: red }')).toBe('.a { font-size: calc(14px * var(--text-scale, 1)); color: red }')
    expect(run('.a { font-size: clamp(18px, 1.6vw, 30px) }')).toBe('.a { font-size: calc(clamp(18px, 1.6vw, 30px) * var(--text-scale, 1)) }')
  })

  it('leaves a size alone that follows its parent or is scaled already', () => {
    for (const value of ['inherit', '1.2em', '90%', 'calc(12px * var(--text-scale, 1))']) {
      expect(run(`.a { font-size: ${value} }`)).toBe(`.a { font-size: ${value} }`)
    }
    expect(run('.a { line-height: 14px }')).toBe('.a { line-height: 14px }')
  })

  it('gives what reduced motion stills to somebody who asked the app for it', () => {
    expect(run('@media (prefers-reduced-motion: reduce) { .a, .b .c { animation: none } :root { --x: 1 } }')).toBe(
      '@media (prefers-reduced-motion: reduce) { .a, .b .c { animation: none } :root { --x: 1 } } '
      + ':root[data-motion="reduced"] .a, :root[data-motion="reduced"] .b .c { animation: none } :root[data-motion="reduced"] { --x: 1 }',
    )
    // The copies keep the order they were written in.
    expect(run('@media (prefers-reduced-motion: reduce) { .a { x: 1 } .b { x: 2 } } .z { x: 3 }')).toBe(
      '@media (prefers-reduced-motion: reduce) { .a { x: 1 } .b { x: 2 } } :root[data-motion="reduced"] .a { x: 1 } :root[data-motion="reduced"] .b { x: 2 } .z { x: 3 }',
    )
  })

  it('copies nothing from any other media rule', () => {
    const css = '@media (min-width: 900px) { .a { display: none } } @media (prefers-reduced-motion: no-preference) { .a { animation: spin 1s } }'
    expect(run(css)).toBe(css)
  })
})

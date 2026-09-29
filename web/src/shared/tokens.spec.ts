import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const kebab = (s: string) => s.replace(/([a-z0-9])([A-Z])/g, '$1-$2').replace(/([a-z])(\d)/g, '$1-$2').toLowerCase()

describe('design tokens', () => {
  it('tokens.css mirrors design/tokens.json', () => {
    const json = JSON.parse(readFileSync(resolve(__dirname, '../../../design/tokens.json'), 'utf8')) as Record<string, unknown>
    const css = readFileSync(resolve(__dirname, 'tokens.css'), 'utf8')
    const groups: Record<string, string> = { color: 'color', font: 'font', radius: 'radius', size: 'size', motion: 'motion' }
    for (const [group, prefix] of Object.entries(groups)) {
      const values = json[group] as Record<string, string>
      for (const [name, value] of Object.entries(values)) {
        expect(css, `${group}.${name}`).toContain(`--${prefix}-${kebab(name)}: ${value};`)
      }
    }
  })
})

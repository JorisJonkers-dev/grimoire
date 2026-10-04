import type { AtRule, Plugin, Rule } from 'postcss'

// A size that follows its parent scales with it, and one that names the scale is scaled already.
const FOLLOWS = /^(inherit|initial|unset)$|(^|[^r])em$|%$|--text-scale/
const REDUCE = /prefers-reduced-motion:\s*reduce/
const ASKED = ':root[data-motion="reduced"]'
const asked = (selector: string) => (selector.startsWith(':root') ? `${ASKED}${selector.slice(5)}` : `${ASKED} ${selector}`)

/**
 * Makes every text size follow the chosen text size, and gives what a device's reduced motion stills
 * to somebody who asked the app for it instead.
 */
export function accessibleCss(): Plugin {
  return {
    postcssPlugin: 'grimoire-accessible-css',
    Once(root) {
      root.walkDecls('font-size', (decl) => {
        if (!FOLLOWS.test(decl.value)) decl.value = `calc(${decl.value} * var(--text-scale, 1))`
      })
      const still: AtRule[] = []
      root.walkAtRules('media', (at) => {
        if (REDUCE.test(at.params)) still.push(at)
      })
      for (const at of still) {
        const copies: Rule[] = []
        at.walkRules((rule) => {
          copies.push(rule.clone({ selectors: rule.selectors.map(asked) }))
        })
        at.after(copies)
      }
    },
  }
}

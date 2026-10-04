// Keyboard shortcuts are declared where they act: a control names its key in `aria-keyshortcuts`,
// and pressing that key presses the control. The page's list of keys is read from the same place.

const NAMED = new Set(['Escape', 'Enter', 'ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'])
const QUIET = new Set(['Shift', 'Control', 'Alt', 'Meta'])
const FIELDS = ['INPUT', 'TEXTAREA', 'SELECT']

const typing = (target: EventTarget | null) =>
  target instanceof Element && (FIELDS.includes(target.tagName) || target.closest('[contenteditable="true"]') !== null)

/** The shortcut a key press is, such as "E", "Shift+1", "?" or "Escape"; none while typing, save Escape, or with Ctrl, Alt or the command key. */
export function keyOf(e: KeyboardEvent): string | null {
  if (e.ctrlKey || e.metaKey || e.altKey || QUIET.has(e.key)) return null
  const digit = /^Digit(\d)$/.exec(e.code)?.[1]
  const plain = digit ?? (NAMED.has(e.key) ? e.key : e.key.toUpperCase())
  if (typing(e.target) && plain !== 'Escape') return null
  // A symbol such as "?" already says Shift was held; a letter or a digit does not.
  const lettered = digit !== undefined || /^[A-Z]$/.test(plain)
  return e.shiftKey && lettered ? `Shift+${plain}` : plain
}

function usable(el: Element): boolean {
  if ((el as HTMLButtonElement).disabled || el.closest('[hidden], [inert]')) return false
  return typeof el.checkVisibility === 'function' ? el.checkVisibility() : true
}
const keysOf = (el: Element) => (el.getAttribute('aria-keyshortcuts') ?? '').split(/\s+/).filter(Boolean)
const answering = (root: ParentNode) => [...root.querySelectorAll('[aria-keyshortcuts]')].filter(usable)

/**
 * Presses the first control that answers to a key. A field takes the caret and a hex of the map the
 * focus, since pressing a hex would walk there; anything else is clicked.
 */
export function press(key: string, root: ParentNode = document): boolean {
  const el = answering(root).find((c) => keysOf(c).some((k) => k.toLowerCase() === key.toLowerCase()))
  if (!el) return false
  const found = el as HTMLElement
  if (FIELDS.includes(el.tagName) || el.hasAttribute('data-hex')) found.focus()
  else found.click()
  return true
}

// What a control is called: its label, the label a field stands in, or its own words.
const named = (el: Element) => el.getAttribute('aria-label') ?? (el as HTMLInputElement).labels?.[0]?.textContent ?? el.textContent

/** What the page answers to now: each key once, by what its control is called. */
export function listed(root: ParentNode = document): { keys: string; what: string }[] {
  const rows = answering(root).map((el) => ({
    keys: keysOf(el).join(' or '),
    what: named(el).trim().replace(/\s+/g, ' '),
  }))
  return rows.filter((row, i) => rows.findIndex((r) => r.keys === row.keys) === i)
}

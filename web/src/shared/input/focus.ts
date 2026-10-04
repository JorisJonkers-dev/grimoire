// Moving the focus without a mouse or the Tab key: for a gamepad's D-pad and bumpers, and for the
// arrows on the map.

const TABBABLE = 'a[href], button, input, select, textarea, [tabindex]'
const REGIONS = 'main, nav, aside, header, footer, form, section[aria-label], [role="group"][aria-label]'

function reachable(el: Element): boolean {
  if ((el as HTMLButtonElement).disabled || el.getAttribute('tabindex') === '-1' || el.closest('[hidden], [inert]')) return false
  return typeof el.checkVisibility === 'function' ? el.checkVisibility() : true
}
// The map is one stop: its hexes are walked with the arrows, not one by one.
const stops = (root: ParentNode) => [...root.querySelectorAll(TABBABLE)].filter((el) => reachable(el) && !el.hasAttribute('data-hex'))
const before = (a: Element, b: Element) => (a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING) !== 0
const focus = (el: Element | undefined) => { (el as HTMLElement | undefined)?.focus() }

/** Moves the focus to the next or previous thing that can take it, stopping at the ends. */
export function stepFocus(dir: 1 | -1) {
  const all = stops(document)
  const at = document.activeElement
  const here = at ? all.indexOf(at) : -1
  if (here >= 0) focus(all[here + dir] ?? at ?? undefined)
  else if (!at || at === document.body) focus(all[0])
  else focus(dir === 1 ? all.find((el) => before(at, el)) : all.findLast((el) => before(el, at)))
}

/** Jumps to the first thing that can take the focus in the next or previous region of the page. */
export function stepRegion(dir: 1 | -1) {
  const firsts = [...document.querySelectorAll(REGIONS)].map((region) => stops(region)[0]).filter((el): el is Element => el !== undefined)
  const starts = firsts.filter((el, i) => firsts.indexOf(el) === i)
  const at = document.activeElement
  const here = at && at !== document.body ? starts.findLastIndex((el) => el === at || before(el, at)) : -1
  focus(starts[here + dir])
}

const STEPS: Record<string, (q: number, r: number, even: boolean) => [number, number][]> = {
  ArrowLeft: (q, r) => [[q - 1, r]],
  ArrowRight: (q, r) => [[q + 1, r]],
  // A row of hexes is shifted half a hex from the one above, so going straight up or down zigzags
  // about a line; where the hex that way is off the map, the other one in that row is taken.
  ArrowUp: (q, r, even) => (even ? [[q, r - 1], [q + 1, r - 1]] : [[q + 1, r - 1], [q, r - 1]]),
  ArrowDown: (q, r, even) => (even ? [[q - 1, r + 1], [q, r + 1]] : [[q, r + 1], [q - 1, r + 1]]),
}

/** With the focus on a hex of the map, moves it to the neighbouring hex an arrow points at; says whether it did. */
export function stepHex(key: string): boolean {
  const at = document.activeElement
  const [q, r] = (at?.getAttribute('data-hex') ?? '').split(',').map(Number)
  const step = STEPS[key]
  if (!at || !step || q === undefined || r === undefined || Number.isNaN(q) || Number.isNaN(r)) return false
  const board = at.closest('svg') ?? document
  for (const [nq, nr] of step(q, r, r % 2 === 0)) {
    const next = board.querySelector(`[data-hex="${String(nq)},${String(nr)}"]`)
    if (next) {
      focus(next)
      return true
    }
  }
  return false
}

import { afterEach, describe, expect, it } from 'vitest'
import { stepFocus, stepHex, stepRegion } from './focus'
import { padKeys } from './gamepad'
import { keyOf, listed, press } from './shortcuts'

const page = (html: string) => {
  document.body.innerHTML = html
  return document.body
}
const key = (init: KeyboardEventInit, on: Element = document.body) => {
  const e = new KeyboardEvent('keydown', { bubbles: true, ...init })
  Object.defineProperty(e, 'target', { value: on })
  return keyOf(e)
}
afterEach(() => {
  document.body.innerHTML = ''
})

describe('which shortcut a key press is', () => {
  it('names letters, digits with Shift, and the keys that have names', () => {
    expect(key({ key: 'e' })).toBe('E')
    expect(key({ key: 'E', shiftKey: true })).toBe('Shift+E')
    expect(key({ key: '!', code: 'Digit1', shiftKey: true })).toBe('Shift+1')
    expect(key({ key: '1', code: 'Digit1' })).toBe('1')
    expect(key({ key: '?', code: 'Slash', shiftKey: true })).toBe('?')
    expect(key({ key: '/', code: 'Slash' })).toBe('/')
    for (const named of ['Escape', 'Enter', 'ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight']) expect(key({ key: named })).toBe(named)
    // A key that only modifies is no shortcut.
    for (const quiet of ['Shift', 'Control', 'Alt', 'Meta']) expect(key({ key: quiet })).toBeNull()
  })

  it('is none while typing, except Escape, and none with Ctrl, Alt or the command key', () => {
    const root = page('<input id="a" /><textarea id="b"></textarea><select id="c"></select><p id="d" contenteditable="true"></p><button id="e"></button>')
    for (const id of ['a', 'b', 'c', 'd']) {
      const field = root.querySelector(`#${id}`) as Element
      expect(key({ key: 'e' }, field), id).toBeNull()
      expect(key({ key: 'Escape' }, field), id).toBe('Escape')
    }
    expect(key({ key: 'e' }, root.querySelector('#e') as Element)).toBe('E')
    expect(key({ key: 'e', ctrlKey: true })).toBeNull()
    expect(key({ key: 'e', metaKey: true })).toBeNull()
    expect(key({ key: 'e', altKey: true })).toBeNull()
  })
})

describe('pressing a shortcut', () => {
  it('presses the first control on the page that answers to it and can be pressed', () => {
    const clicked: string[] = []
    const root = page(`
      <button id="off" aria-keyshortcuts="E" disabled>End turn</button>
      <div hidden><button id="hidden" aria-keyshortcuts="E">Hidden</button></div>
      <button id="end" aria-keyshortcuts="E">End turn</button>
      <button id="later" aria-keyshortcuts="E">Later</button>
      <button id="both" aria-keyshortcuts="C Shift+C">Confirm</button>
      <input id="seek" aria-keyshortcuts="/" aria-label="Search everything" />`)
    root.querySelectorAll('button').forEach((b) => { b.addEventListener('click', () => clicked.push(b.id)) })
    expect(press('E')).toBe(true)
    expect(press('e')).toBe(true)
    expect(press('Shift+C')).toBe(true)
    expect(press('C')).toBe(true)
    expect(clicked).toEqual(['end', 'end', 'both', 'both'])
    // A field is not clicked: the caret goes to it.
    expect(press('/')).toBe(true)
    expect(document.activeElement?.id).toBe('seek')
    expect(press('Q')).toBe(false)
    expect(press('Shift+E')).toBe(false)
    expect(clicked).toHaveLength(4)
  })

  it('lists what the page answers to, each key once, by what its control is called', () => {
    page(`
      <button aria-keyshortcuts="E">End turn</button>
      <button aria-keyshortcuts="E">End turn</button>
      <button aria-keyshortcuts="C" aria-label="Confirm the walk">Go</button>
      <button aria-keyshortcuts="Escape" disabled>  Cancel  </button>
      <div hidden><button aria-keyshortcuts="X">Hidden</button></div>
      <input aria-keyshortcuts="/" aria-label="Search everything" />`)
    expect(listed()).toEqual([
      { keys: 'E', what: 'End turn' },
      { keys: 'C', what: 'Confirm the walk' },
      { keys: '/', what: 'Search everything' },
    ])
  })
})

describe('moving the focus', () => {
  it('steps through what can take the focus, in order, and stops at the ends', () => {
    const root = page('<a href="/x" id="a">A</a><button id="b">B</button><button disabled id="no">no</button><input id="c" /><span tabindex="-1" id="skip">s</span><div tabindex="0" id="d">D</div><div hidden><button id="h">h</button></div>')
    const at = () => document.activeElement?.id
    // With nothing focused, the first step lands on the first.
    stepFocus(1)
    expect(at()).toBe('a')
    stepFocus(1)
    stepFocus(1)
    expect(at()).toBe('c')
    stepFocus(1)
    expect(at()).toBe('d')
    stepFocus(1)
    expect(at()).toBe('d')
    stepFocus(-1)
    expect(at()).toBe('c')
    ;(root.querySelector('#a') as HTMLElement).focus()
    stepFocus(-1)
    expect(at()).toBe('a')
  })

  it('starts from the first whichever way it is asked to go, and passes the map as one stop', () => {
    const root = page('<button id="a">A</button><svg><g role="button" tabindex="0" data-hex="0,0"></g><g role="button" tabindex="0" data-hex="1,0"></g></svg><button id="b">B</button>')
    const at = () => document.activeElement?.id ?? ''
    stepFocus(-1)
    expect(at()).toBe('a')
    // The hexes are walked with the arrows; stepping the focus goes past them.
    stepFocus(1)
    expect(at()).toBe('b')
    stepFocus(-1)
    expect(at()).toBe('a')
    // From a hex, the step is to what comes after or before the map.
    ;(root.querySelector('[data-hex="1,0"]') as HTMLElement).focus()
    stepFocus(1)
    expect(at()).toBe('b')
    ;(root.querySelector('[data-hex="0,0"]') as HTMLElement).focus()
    stepFocus(-1)
    expect(at()).toBe('a')
  })

  it('jumps between the regions of a page', () => {
    page(`
      <nav aria-label="Main"><a href="/a" id="n1">a</a><a href="/b" id="n2">b</a></nav>
      <main><button id="m1">one</button>
        <section aria-label="Empty"></section>
        <section aria-label="Bars"><button id="s1">x</button><button id="s2">y</button></section>
      </main>
      <footer><a href="/f" id="f1">f</a></footer>`)
    const at = () => document.activeElement?.id
    stepRegion(1)
    expect(at()).toBe('n1')
    stepRegion(1)
    expect(at()).toBe('m1')
    // A region with nothing to focus is passed over.
    stepRegion(1)
    expect(at()).toBe('s1')
    stepRegion(1)
    expect(at()).toBe('f1')
    stepRegion(1)
    expect(at()).toBe('f1')
    stepRegion(-1)
    expect(at()).toBe('s1')
    ;(document.querySelector('#s2') as HTMLElement).focus()
    stepRegion(-1)
    expect(at()).toBe('m1')
  })

  it('steps from hex to hex with the arrows, keeping a straight line up and down', () => {
    const hexes = ['0,0', '1,0', '-1,0', '0,-1', '1,-1', '0,1', '-1,1', '0,-2', '-1,2', '2,0']
    page(`<svg>${hexes.map((h) => `<g role="button" tabindex="0" data-hex="${h}"></g>`).join('')}</svg><button id="b">b</button>`)
    const at = () => document.activeElement?.getAttribute('data-hex')
    const go = (...keys: string[]) => keys.map((k) => stepHex(k))
    ;(document.querySelector('[data-hex="0,0"]') as HTMLElement).focus()
    go('ArrowRight')
    expect(at()).toBe('1,0')
    go('ArrowLeft', 'ArrowLeft')
    expect(at()).toBe('-1,0')
    go('ArrowRight')
    // Up and down zigzag about a line, and come back the way they went.
    go('ArrowUp')
    expect(at()).toBe('0,-1')
    go('ArrowUp')
    expect(at()).toBe('0,-2')
    go('ArrowDown', 'ArrowDown')
    expect(at()).toBe('0,0')
    go('ArrowDown')
    expect(at()).toBe('-1,1')
    go('ArrowDown')
    expect(at()).toBe('-1,2')
    go('ArrowUp', 'ArrowUp')
    expect(at()).toBe('0,0')
    // At the edge the other hex that way is taken, and with none the focus stays.
    ;(document.querySelector('[data-hex="2,0"]') as HTMLElement).focus()
    expect(go('ArrowUp')).toEqual([false])
    ;(document.querySelector('[data-hex="1,0"]') as HTMLElement).focus()
    expect(go('ArrowUp')).toEqual([true])
    expect(at()).toBe('1,-1')
    expect(go('ArrowUp', 'ArrowRight')).toEqual([false, false])
    expect(at()).toBe('1,-1')
    // Off the map, the arrows are not the map's.
    ;(document.querySelector('#b') as HTMLElement).focus()
    expect(go('ArrowLeft')).toEqual([false])
    expect(stepHex('E')).toBe(false)
  })
})

describe('a gamepad', () => {
  const pad = (pressed: number[], axes: number[] = [0, 0]) => ({ buttons: Array.from({ length: 16 }, (_, i) => pressed.includes(i)), axes })
  const none = pad([])

  it('presses the keys its buttons stand for, once for each press', () => {
    expect(padKeys(none, pad([0]))).toEqual(['Enter'])
    expect(padKeys(none, pad([1, 2, 3]))).toEqual(['Escape', 'E', 'M'])
    expect(padKeys(none, pad([4, 5, 6, 7]))).toEqual(['PreviousRegion', 'NextRegion', 'C', 'R'])
    expect(padKeys(none, pad([8]))).toEqual(['?'])
    expect(padKeys(none, pad([9]))).toEqual(['?'])
    expect(padKeys(none, pad([12, 13, 14, 15]))).toEqual(['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'])
    // Held down, a button is one press; a button with no key is none.
    expect(padKeys(pad([0]), pad([0, 2]))).toEqual(['E'])
    expect(padKeys(pad([0]), pad([0]))).toEqual([])
    expect(padKeys(none, pad([10, 11]))).toEqual([])
    expect(padKeys(pad([0]), none)).toEqual([])
  })

  it('takes the left stick for the arrows, past halfway and once for each push', () => {
    expect(padKeys(none, pad([], [-0.9, 0]))).toEqual(['ArrowLeft'])
    expect(padKeys(none, pad([], [0.9, 0]))).toEqual(['ArrowRight'])
    expect(padKeys(none, pad([], [0, -0.9]))).toEqual(['ArrowUp'])
    expect(padKeys(none, pad([], [0, 0.9]))).toEqual(['ArrowDown'])
    expect(padKeys(none, pad([], [0.5, -0.5]))).toEqual([])
    expect(padKeys(none, pad([], [0.51, 0]))).toEqual(['ArrowRight'])
    expect(padKeys(pad([], [0.9, 0]), pad([], [0.8, 0]))).toEqual([])
    expect(padKeys(pad([], [0.9, 0]), pad([], [-0.8, 0]))).toEqual(['ArrowLeft'])
    // A pad with no stick has no axes.
    expect(padKeys({ buttons: [], axes: [] }, { buttons: [true], axes: [] })).toEqual(['Enter'])
  })
})

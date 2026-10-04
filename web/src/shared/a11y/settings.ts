import { reactive } from 'vue'

export const palettes = ['standard', 'red-green', 'blue-yellow'] as const
export const textSizes = ['normal', 'large', 'largest'] as const
const flags = ['dyslexiaFont', 'reduceMotion', 'haptics', 'announceTurns', 'announceRolls'] as const
export type Palette = (typeof palettes)[number]
export type TextSize = (typeof textSizes)[number]
export type Accessibility = { palette: Palette; textSize: TextSize } & Record<(typeof flags)[number], boolean>

const KEY = 'grimoire.accessibility'
const standard = (): Accessibility => ({ palette: 'standard', textSize: 'normal', dyslexiaFont: false, reduceMotion: false, haptics: true, announceTurns: true, announceRolls: true })

/** How this device is set; it is kept on the device, since a phone and a TV want different things. */
export const accessibility = reactive(standard())

function kept(): Record<string, unknown> {
  try {
    const was: unknown = JSON.parse(localStorage.getItem(KEY) ?? 'null')
    return typeof was === 'object' && was !== null ? (was as Record<string, unknown>) : {}
  } catch {
    return {}
  }
}
// The stylesheet reads the look from the page's root.
function apply() {
  const root = document.documentElement.dataset
  root.palette = accessibility.palette
  root.text = accessibility.textSize
  root.font = accessibility.dyslexiaFont ? 'dyslexia' : 'standard'
  root.motion = accessibility.reduceMotion ? 'reduced' : 'system'
}

/** Takes up what this device kept, leaving out anything it does not know. */
export function restoreAccessibility() {
  const was = kept()
  const base = standard()
  accessibility.palette = palettes.find((p) => p === was.palette) ?? base.palette
  accessibility.textSize = textSizes.find((s) => s === was.textSize) ?? base.textSize
  for (const flag of flags) {
    const value = was[flag]
    accessibility[flag] = typeof value === 'boolean' ? value : base[flag]
  }
  apply()
}

export function setAccessibility(change: Partial<Accessibility>) {
  Object.assign(accessibility, change)
  apply()
  try {
    localStorage.setItem(KEY, JSON.stringify(accessibility))
  } catch {
    // A browser that keeps nothing still shows the choice until the page is left.
  }
}

export function resetAccessibility() {
  Object.assign(accessibility, standard())
  apply()
  try {
    localStorage.removeItem(KEY)
  } catch {
    // Nothing was kept.
  }
}

export const deviceReducesMotion = () => typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
export const reducedMotion = () => accessibility.reduceMotion || deviceReducesMotion()

export const canBuzz = () => 'vibrate' in navigator
/** Vibrates the device, while haptics are on and where it can. */
export function buzz(pattern: number | number[]) {
  if (accessibility.haptics && canBuzz()) navigator.vibrate(pattern)
}

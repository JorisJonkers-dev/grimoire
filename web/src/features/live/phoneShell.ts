import { computed, ref } from 'vue'

/** The pages a player swipes between on a phone, in order; Map shows the map alone. */
export const PAGES = [
  { key: 'map', label: 'Map' },
  { key: 'actions', label: 'Actions' },
  { key: 'spells', label: 'Spells' },
  { key: 'character', label: 'Character' },
  { key: 'party', label: 'Party' },
] as const
export type PageKey = (typeof PAGES)[number]['key']

/** How far a finger travels sideways before a swipe turns the page. */
export const SWIPE_PX = 60
/** How little a finger may move for a touch to count as a tap. */
export const TAP_PX = 10
export const ZOOM_MIN = 0.5
export const ZOOM_MAX = 3
export const ZOOM_STEP = 0.5

/** The page a swipe leads to: left for the next, right for the one before; a mostly vertical drag is a scroll. */
export function pageAfterSwipe(page: PageKey, dx: number, dy: number): PageKey {
  if (Math.abs(dx) < SWIPE_PX || Math.abs(dx) < 2 * Math.abs(dy)) return page
  const at = PAGES.findIndex((p) => p.key === page)
  return PAGES[Math.min(PAGES.length - 1, Math.max(0, at + (dx < 0 ? 1 : -1)))]?.key ?? page
}

const clamp = (z: number) => Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, Math.round(z * 100) / 100))

/** The zoom a pinch leaves: the zoom it began at, scaled by how far the fingers have spread. */
export function pinched(start: number, startDistance: number, distance: number): number {
  return startDistance > 0 ? clamp((start * distance) / startDistance) : start
}

type Point = { x: number; y: number }
const point = (e: PointerEvent): Point => ({ x: e.clientX, y: e.clientY })

/** The phone shell's state: the page shown, swipes that turn it, and the map's zoom by pinch or button. */
export function usePhoneShell() {
  const page = ref<PageKey>('map')
  const zoom = ref(1)
  let swipe: { id: number; from: Point } | undefined
  const fingers = new Map<number, Point>()
  let pinch: { zoom: number; distance: number } | undefined

  const spread = () => {
    const [a, b] = [...fingers.values()]
    return a && b ? Math.hypot(a.x - b.x, a.y - b.y) : 0
  }
  function swipeStart(e: PointerEvent) {
    if (e.pointerType === 'touch') swipe = { id: e.pointerId, from: point(e) }
  }
  // A still touch on a page's button turns to it at once: right after a swipe the browser can swallow
  // the click that would.
  function swipeEnd(e: PointerEvent) {
    if (swipe?.id === e.pointerId) {
      const [dx, dy] = [e.clientX - swipe.from.x, e.clientY - swipe.from.y]
      const still = Math.abs(dx) < TAP_PX && Math.abs(dy) < TAP_PX
      const tapped = still && e.target instanceof Element ? e.target.closest('[data-page-key]')?.getAttribute('data-page-key') : undefined
      page.value = PAGES.find((p) => p.key === tapped)?.key ?? pageAfterSwipe(page.value, dx, dy)
    }
    swipe = undefined
  }
  function pinchStart(e: PointerEvent) {
    if (e.pointerType !== 'touch') return
    fingers.set(e.pointerId, point(e))
    pinch = fingers.size === 2 ? { zoom: zoom.value, distance: spread() } : undefined
  }
  function pinchMove(e: PointerEvent) {
    if (!fingers.has(e.pointerId)) return
    fingers.set(e.pointerId, point(e))
    if (pinch) zoom.value = pinched(pinch.zoom, pinch.distance, spread())
  }
  function pinchEnd(e: PointerEvent) {
    fingers.delete(e.pointerId)
    pinch = undefined
  }
  const step = (by: number) => { zoom.value = clamp(zoom.value + by) }
  return {
    page, zoom, swipeStart, swipeEnd, pinchStart, pinchMove, pinchEnd,
    zoomIn: () => { step(ZOOM_STEP) },
    zoomOut: () => { step(-ZOOM_STEP) },
    zoomReset: () => { zoom.value = 1 },
    canZoomIn: computed(() => zoom.value < ZOOM_MAX),
    canZoomOut: computed(() => zoom.value > ZOOM_MIN),
  }
}

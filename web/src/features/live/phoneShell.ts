import { computed, ref } from 'vue'

export type PageKey = 'map' | 'actions' | 'spells' | 'character' | 'party' | 'table' | 'tools'
export type Page = { key: PageKey; label: string }
/** The pages a player swipes between on a phone, in order; Map shows the map alone. */
export const PLAYER_PAGES: readonly Page[] = [
  { key: 'map', label: 'Map' },
  { key: 'actions', label: 'Actions' },
  { key: 'spells', label: 'Spells' },
  { key: 'character', label: 'Character' },
  { key: 'party', label: 'Party' },
]
/** The DM's remote: the creatures in hand, the Table Display, the map tools, and the party's things. */
export const DM_PAGES: readonly Page[] = [
  { key: 'map', label: 'Map' },
  { key: 'actions', label: 'Creatures' },
  { key: 'table', label: 'Table' },
  { key: 'tools', label: 'Tools' },
  { key: 'party', label: 'Party' },
]

/** How far a finger travels sideways before a swipe turns the page. */
export const SWIPE_PX = 60
/** How little a finger may move for a touch to count as a tap. */
export const TAP_PX = 10
export const ZOOM_MIN = 0.5
export const ZOOM_MAX = 3
export const ZOOM_STEP = 0.5

/** The page a swipe leads to: left for the next, right for the one before; a mostly vertical drag is a scroll. */
export function pageAfterSwipe(pages: readonly Page[], page: PageKey, dx: number, dy: number): PageKey {
  const at = pages.findIndex((p) => p.key === page)
  if (at < 0 || Math.abs(dx) < SWIPE_PX || Math.abs(dx) < 2 * Math.abs(dy)) return page
  return pages[Math.min(pages.length - 1, Math.max(0, at + (dx < 0 ? 1 : -1)))]?.key ?? page
}

const clamp = (z: number) => Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, Math.round(z * 100) / 100))

/** The zoom a pinch leaves: the zoom it began at, scaled by how far the fingers have spread. */
export function pinched(start: number, startDistance: number, distance: number): number {
  return startDistance > 0 ? clamp((start * distance) / startDistance) : start
}

type Point = { x: number; y: number }
const point = (e: PointerEvent): Point => ({ x: e.clientX, y: e.clientY })

/** The phone shell's state: the page shown, swipes that turn it, and the map's zoom by pinch or button. */
export function usePhoneShell(pages: () => readonly Page[]) {
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
      page.value = pages().find((p) => p.key === tapped)?.key ?? pageAfterSwipe(pages(), page.value, dx, dy)
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

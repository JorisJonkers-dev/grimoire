// A picture the API serves, such as a Map's image, is named by a path under /api/v1/. In the demo there
// is no API to serve it, and an <img> or an SVG <image> fetches on its own, past the demo fetch. So the
// demo swaps such a path for a drawn stand-in before the element can ask for it.

const api = /^\/api\/v1\//

// Flagstones under shallow water: the seeded Maps are a drowned chapel and its shore.
const drawing = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" preserveAspectRatio="none">
<defs><pattern id="s" width="16" height="16" patternUnits="userSpaceOnUse">
<rect width="16" height="16" fill="#3b4a45"/><path d="M0 0H16V16" fill="none" stroke="#2a3632" stroke-width="1.2"/></pattern></defs>
<rect width="64" height="64" fill="url(#s)"/><rect width="64" height="64" fill="#5d7f86" opacity=".25"/></svg>`

/** What the demo shows in place of a picture at an API path. */
export const standIn = `data:image/svg+xml,${encodeURIComponent(drawing)}`

/** The address an element may load: an API path becomes the stand-in, anything else stays. */
export function demoSource(value: string): string {
  return api.test(value) ? standIn : value
}

const watched = new Set(['src', 'href'])

/** Routes every picture at an API path to the stand-in, for as long as the demo runs. */
export function guardPictures(): () => void {
  const setAttribute = Object.getOwnPropertyDescriptor(Element.prototype, 'setAttribute')?.value as (this: Element, name: string, value: string) => void
  // The DOM always defines an image's src as an accessor.
  const src = Object.getOwnPropertyDescriptor(HTMLImageElement.prototype, 'src') as PropertyDescriptor & { set: (value: string) => void }
  Element.prototype.setAttribute = function demoSetAttribute(this: Element, name: string, value: string) {
    setAttribute.call(this, name, watched.has(name) ? demoSource(value) : value)
  }
  Object.defineProperty(HTMLImageElement.prototype, 'src', {
    ...src,
    set(this: HTMLImageElement, value: string) {
      src.set.call(this, demoSource(value))
    },
  })
  return () => {
    Element.prototype.setAttribute = setAttribute
    Object.defineProperty(HTMLImageElement.prototype, 'src', src)
  }
}

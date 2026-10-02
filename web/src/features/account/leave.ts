const NEXT = 'grimoire.oidc.next'

/** Sends the browser to the external login, remembering where to return inside Grimoire. */
export function leaveFor(url: string, next = '/') {
  try {
    sessionStorage.setItem(NEXT, next)
  } catch {
    // Without storage the person lands on the home page afterwards.
  }
  window.location.assign(url)
}

/** Where to go after an external sign-in: a path inside Grimoire, or home. */
export function returnTo() {
  let n: string | null = null
  try {
    n = sessionStorage.getItem(NEXT)
    sessionStorage.removeItem(NEXT)
  } catch {
    n = null
  }
  return n?.startsWith('/') && !n.startsWith('//') ? n : '/'
}

import { allOperations, DemoProblem, routeOf, type Handlers, type Route } from './operations'
import type { Viewer } from './role'
import type { World } from './world'

function json(body: unknown, status: number): Response {
  const type = status >= 400 ? 'application/problem+json' : 'application/json'
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': type } })
}

function problem(status: number, title: string): Response {
  return json({ type: 'about:blank', title, status }, status)
}

// A path or a query arrives as text; the contract may ask for a number.
function loosen(value: string): unknown {
  return /^-?\d+(\.\d+)?$/.test(value) ? Number(value) : value
}

function queryOf(url: URL, typed: boolean): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const key of new Set(url.searchParams.keys())) {
    const all = url.searchParams.getAll(key).map((v) => (typed ? loosen(v) : v))
    out[key] = all.length > 1 ? all : all[0]
  }
  return out
}

async function bodyOf(request: Request): Promise<unknown> {
  const text = await request.text()
  if (!text) return undefined
  try {
    return JSON.parse(text) as unknown
  } catch {
    return text
  }
}

type Parsed = { path?: Record<string, unknown>; query?: Record<string, unknown>; body?: unknown }

/**
 * The request as the SDK's own schema reads it: "3" in a path or a query is the number 3 where the contract asks
 * for a number and the text "3" where it asks for text. Null when the request breaks the contract,
 * which the API refuses too.
 */
async function argumentsFor(route: Route, url: URL, path: Record<string, string>, body: unknown): Promise<Parsed | null> {
  const shape = (typed: boolean): Parsed => ({
    ...(route.params.length > 0 ? { path: typed ? Object.fromEntries(Object.entries(path).map(([k, v]) => [k, loosen(v)])) : path } : {}),
    ...(url.search ? { query: queryOf(url, typed) } : {}),
    ...(body === undefined ? {} : { body }),
  })
  for (const typed of [false, true]) {
    try {
      return (await route.validate(shape(typed))) as Parsed
    } catch {
      // Read it the other way next.
    }
  }
  return null
}

export type DemoFetchOptions = {
  world: () => World
  viewer: () => Viewer
  handlers: Handlers
  /** Told of every request no handler answers, so a test can insist there are none. */
  onUnhandled: (what: string) => void
  /** Told of every answer, so a test can insist a page met no refusal. */
  onAnswer: (what: string, status: number) => void
}

// Where a literal segment and a path parameter could both match, the literal wins, as in the API's
// router: /library/shared is not the Library Entry called "shared".
function literalFirst(a: Route, b: Route): number {
  const as = a.template.split('/')
  const bs = b.template.split('/')
  for (let i = 0; i < Math.min(as.length, bs.length); i++) {
    const aParam = String(as[i]).startsWith('{')
    const bParam = String(bs[i]).startsWith('{')
    if (aParam !== bParam) return aParam ? 1 : -1
  }
  return 0
}

/**
 * A fetch that never leaves the browser: every API call is answered from the in-memory world by the
 * handler for its operation, and anything else is refused as the server would refuse an unknown path.
 */
export function createDemoFetch(options: DemoFetchOptions): typeof fetch {
  const routes = allOperations().map(routeOf).sort(literalFirst)
  return async (input, init) => {
    const request = input instanceof Request ? input : new Request(input, init)
    const url = new URL(request.url)
    const method = request.method.toUpperCase()
    for (const route of routes) {
      if (route.method !== method) continue
      const match = route.pattern.exec(url.pathname)
      if (!match) continue
      const handler = options.handlers[route.operation] as ((call: unknown) => unknown) | undefined
      if (!handler) {
        options.onUnhandled(`${route.operation} ${method} ${url.pathname}`)
        return problem(501, 'Not available in the demo')
      }
      const path = Object.fromEntries(route.params.map((name, i) => [name, decodeURIComponent(String(match[i + 1]))]))
      const what = `${route.operation} ${method} ${url.pathname}`
      const parsed = await argumentsFor(route, url, path, await bodyOf(request))
      if (!parsed) {
        options.onAnswer(what, 400)
        return problem(400, 'The request does not match the contract')
      }
      try {
        const reply = handler({ ...parsed, world: options.world(), who: options.viewer() })
        options.onAnswer(what, reply === undefined ? 204 : 200)
        return reply === undefined ? new Response(null, { status: 204 }) : json(reply, 200)
      } catch (error) {
        if (!(error instanceof DemoProblem)) throw error
        options.onAnswer(what, error.status)
        return json(error.toProblem(), error.status)
      }
    }
    options.onUnhandled(`${method} ${url.pathname}`)
    return problem(404, 'Not found')
  }
}


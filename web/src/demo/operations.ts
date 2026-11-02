import * as sdk from '@/infrastructure/api/sdk.gen'
import type { Problem } from '@/infrastructure/api/types.gen'
import type { World } from './world'
import type { Viewer } from './role'

// Every handler is typed from the generated SDK: its name is an SDK function, its arguments are that
// function's path, query and body, and what it returns is that function's success body. When the
// contract changes and `task gen` rewrites the SDK, a handler that no longer matches fails vue-tsc.

type Sdk = typeof sdk
type Fn = (...args: never[]) => Promise<unknown>

/** The name of an SDK function, which is the operationId in the contract. */
export type OperationId = { [K in keyof Sdk]: Sdk[K] extends Fn ? K : never }[keyof Sdk]

type Options<K extends OperationId> = NonNullable<Parameters<Sdk[K]>[0]>
type Part<K extends OperationId, P extends string> = Options<K> extends { [Q in P]?: infer X } ? X : never

// A required query object is always there once the request passed the SDK's schema; an optional one may not be.
// eslint-disable-next-line @typescript-eslint/no-empty-object-type
type Query<K extends OperationId> = {} extends Pick<Options<K>, 'query' & keyof Options<K>> ? Part<K, 'query'> | undefined : Part<K, 'query'>

/** The success body an operation answers with. */
export type Reply<K extends OperationId> = Extract<Awaited<ReturnType<Sdk[K]>>, { error: undefined }>['data']

export type Call<K extends OperationId> = {
  path: Part<K, 'path'>
  /** Absent when the URL carries no query string, which an operation whose query is all optional allows. */
  query: Query<K>
  body: Part<K, 'body'>
  world: World
  who: Viewer
}

export type Handler<K extends OperationId> = (call: Call<K>) => Reply<K>
export type Handlers = { [K in OperationId]?: Handler<K> }

/** A refusal a handler throws; it reaches the page as the RFC 9457 problem the API would send. */
export class DemoProblem extends Error {
  constructor(readonly status: number, readonly title: string) {
    super(title)
  }

  toProblem(): Problem {
    return { type: 'about:blank', title: this.title, status: this.status }
  }
}

export const notFound = (what: string) => new DemoProblem(404, `${what} not found`)
export const forbidden = () => new DemoProblem(403, 'Forbidden')
export const unauthorized = () => new DemoProblem(401, 'Sign in first')

type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

type Validator = (data: unknown) => Promise<unknown>

export type Route = {
  operation: OperationId
  method: Method
  template: string
  pattern: RegExp
  params: string[]
  /** The SDK's own request schema; the client is generated with one for every operation. */
  validate: Validator
}

type Recorded = { method: Method; url: string; requestValidator: Validator }

// A client that sends nothing: it only notes what an SDK function would have asked for.
function recorder(into: Recorded[]) {
  const take = (method: Method) => (options: { url: string; requestValidator: Validator }) => {
    into.push({ method, url: options.url, requestValidator: options.requestValidator })
    return Promise.resolve({ data: undefined, error: undefined })
  }
  return { get: take('GET'), post: take('POST'), put: take('PUT'), patch: take('PATCH'), delete: take('DELETE') }
}

function escape(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

/** Where each operation lives, read from the generated SDK itself, so no table is kept by hand. */
export function routeOf(operation: OperationId): Route {
  const seen: Recorded[] = []
  const call = sdk[operation] as (options: { client: unknown }) => unknown
  void call({ client: recorder(seen) })
  // Every SDK function sends exactly one request, at once.
  const [first] = seen as [Recorded]
  const params: string[] = []
  const source = first.url
    .split(/(\{[^}]+\})/)
    .map((piece) => {
      const name = /^\{([^}]+)\}$/.exec(piece)?.[1]
      if (!name) return escape(piece)
      params.push(name)
      return '([^/]+)'
    })
    .join('')
  return {
    operation,
    method: first.method,
    template: first.url,
    pattern: new RegExp(`^${source}$`),
    params,
    validate: first.requestValidator,
  }
}

/** Every operation the SDK has. */
export function allOperations(): OperationId[] {
  return (Object.keys(sdk) as (keyof Sdk)[]).filter((k): k is OperationId => typeof sdk[k] === 'function')
}

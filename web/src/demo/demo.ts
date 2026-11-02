import { reactive } from 'vue'
import { createDemoFetch } from './demoFetch'
import { createDemoSocket } from './demoSocket'
import { handlers } from './handlers'
import type { Handlers } from './operations'
import { viewerOf, type Persona } from './role'
import { seedWorld, type World } from './world'

export type Demo = {
  world: World
  fetch: typeof fetch
  /** Stands in for WebSocket, so a live Session runs on the same world. */
  Socket: typeof WebSocket
  /** Requests no handler answered; the demo test insists this stays empty. */
  unhandled: string[]
  /** Every answer given, with its status. */
  answers: { what: string; status: number }[]
}

/** A fresh seeded world, and the fetch and socket that answer from it. */
export function createDemo(persona: Persona = 'player', answering: Handlers = handlers): Demo {
  const world: World = reactive(seedWorld(persona))
  const unhandled: string[] = []
  const answers: Demo['answers'] = []
  const viewer = () => viewerOf(world)
  const fetch = createDemoFetch({
    world: () => world,
    viewer,
    handlers: answering,
    onUnhandled: (what) => {
      unhandled.push(what)
      console.warn(`[demo] no answer for ${what}`)
    },
    onAnswer: (what, status) => answers.push({ what, status }),
  })
  return { world, fetch, Socket: createDemoSocket(() => world, viewer), unhandled, answers }
}

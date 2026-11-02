// Ids for the personal records: the Account's own things, its Friends and its Conversations. They sit in
// a range of their own (…0000ee00xxxx), so they never meet the ids of another part of the world.

export const personalId = (n: number) => `0190c7a8-0000-7000-8000-0000ee00${n.toString(16).padStart(4, '0')}`

let made = 0

/** A new id for something made in the demo; unique for as long as the page is open. */
export function freshId(): string {
  made += 1
  return `0190c7a8-0000-7000-8000-ff${made.toString(16).padStart(10, '0')}`
}

let last = 0

/** The moment a write happens, as the API stamps it; two writes in one millisecond still come in order. */
export function now(): string {
  last = Math.max(Date.now(), last + 1)
  return new Date(last).toISOString()
}

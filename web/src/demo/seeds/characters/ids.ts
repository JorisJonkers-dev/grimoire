// Ids for the Character records the page list never opens directly, in a range of their own so no
// other area of the seeded world collides with them.

export const cid = (tail: string) => `0190c7a8-0000-7000-8c00-${tail.padStart(12, '0')}`

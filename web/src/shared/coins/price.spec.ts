import { describe, expect, it } from 'vitest'
import { formatPrice, haggled } from './price'

describe('prices', () => {
  it('writes copper as gold, silver and copper', () => {
    expect([formatPrice(0), formatPrice(7), formatPrice(150), formatPrice(1234), formatPrice(100)]).toEqual(['0 cp', '7 cp', '1 gp 5 sp', '12 gp 3 sp 4 cp', '1 gp'])
  })

  it('moves an asking price by the haggle, never below a copper', () => {
    expect([haggled(150, -10), haggled(150, 20), haggled(150, 0), haggled(1, -50), haggled(99, -1)]).toEqual([135, 180, 150, 1, 98])
  })
})

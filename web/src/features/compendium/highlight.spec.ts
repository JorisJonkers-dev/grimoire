import { describe, expect, it } from 'vitest'
import { highlight, levelLabel, titleCase } from './highlight'

describe('highlight', () => {
  it('marks whole-word mentions case-insensitively', () => {
    expect(highlight('The target falls Prone and is prone to fear; proneness is not.', ['Prone'])).toEqual([
      { text: 'The target falls ' },
      { text: 'Prone', mention: 'Prone' },
      { text: ' and is ' },
      { text: 'prone', mention: 'Prone' },
      { text: ' to fear; proneness is not.' },
    ])
  })

  it('prefers the longest term and returns plain text without terms', () => {
    expect(highlight('Heavily Obscured area', ['Obscured', 'Heavily Obscured'])[0]).toEqual({
      text: 'Heavily Obscured',
      mention: 'Heavily Obscured',
    })
    expect(highlight('Nothing here', [])).toEqual([{ text: 'Nothing here' }])
    expect(highlight('Nothing here', ['  '])).toEqual([{ text: 'Nothing here' }])
    expect(highlight('prone', ['Prone'])).toEqual([{ text: 'prone', mention: 'Prone' }])
  })
})

describe('labels', () => {
  it('names spell levels', () => {
    expect([0, 1, 2, 3, 4, 9].map(levelLabel)).toEqual(['Cantrip', '1st level', '2nd level', '3rd level', '4th level', '9th level'])
  })
  it('title-cases slugs', () => {
    expect(titleCase('sleight-of-hand')).toBe('Sleight Of Hand')
  })
})

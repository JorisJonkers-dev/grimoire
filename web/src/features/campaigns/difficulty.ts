import type { Difficulty } from '@/infrastructure/api/types.gen'

/** The difficulty presets, easiest first, with what each changes. */
export const difficulties: { value: Difficulty; name: string; changes: string }[] = [
  { value: 'story', name: 'Story', changes: 'enemies come with three quarters of their hit points and attack at −2' },
  { value: 'standard', name: 'Standard', changes: 'the rules as written' },
  { value: 'hard', name: 'Hard', changes: 'enemies come with a quarter more hit points and attack at +2' },
]

export const KARMIC_DICE = 'Karmic dice: after two low d20s in a row, the next d20 the app rolls for you leans high; after two high ones it leans low. A die you throw yourself is never changed.'

/** What a table plays by that every Member should know: a difficulty preset that changes play, and karmic dice. */
export function tableRules(difficulty: Difficulty | undefined, karmicDice: boolean | undefined): string[] {
  const preset = difficulties.find((d) => d.value === difficulty && d.value !== 'standard')
  return [...(preset ? [`${preset.name} difficulty: ${preset.changes}.`] : []), ...(karmicDice ? [KARMIC_DICE] : [])]
}

<script setup lang="ts">
import { GButton } from '@/shared/ui'
import { accessibility, buzz, canBuzz, deviceReducesMotion, resetAccessibility, setAccessibility, type Palette, type TextSize } from '@/shared/a11y/settings'

const colours: { value: Palette; label: string; hint: string }[] = [
  { value: 'standard', label: 'Standard', hint: 'Blue allies, orange enemies, green for what went well.' },
  { value: 'red-green', label: 'Red–green', hint: 'For protanopia and deuteranopia.' },
  { value: 'blue-yellow', label: 'Blue–yellow', hint: 'For tritanopia.' },
]
const sizes: { value: TextSize; label: string }[] = [
  { value: 'normal', label: 'Normal' },
  { value: 'large', label: 'Large' },
  { value: 'largest', label: 'Largest' },
]
const checked = (e: Event) => (e.target as HTMLInputElement).checked
function haptics(on: boolean) {
  setAccessibility({ haptics: on })
  buzz(40)
}
</script>

<template>
  <main class="g-page">
    <header class="g-headline">
      <span class="g-eyebrow">This device</span>
      <h1>Accessibility</h1>
    </header>
    <p class="hint">These are kept on this device, so your phone and the TV can each be set as they need.</p>
    <fieldset class="g-card stack">
      <legend>Colours</legend>
      <label v-for="c in colours" :key="c.value" class="pick">
        <input type="radio" name="palette" :value="c.value" :checked="accessibility.palette === c.value" :data-testid="`palette-${c.value}`" @change="setAccessibility({ palette: c.value })" />
        <span>{{ c.label }} <small>{{ c.hint }}</small></span>
      </label>
      <p class="sample" aria-hidden="true" data-testid="palette-sample">
        <span class="chip chip--party">Ally</span>
        <span class="chip chip--enemy">Enemy</span>
        <span class="chip chip--success">Success</span>
      </p>
    </fieldset>
    <fieldset class="g-card stack">
      <legend>Text</legend>
      <label v-for="s in sizes" :key="s.value" class="pick">
        <input type="radio" name="text-size" :value="s.value" :checked="accessibility.textSize === s.value" :data-testid="`text-${s.value}`" @change="setAccessibility({ textSize: s.value })" />
        <span>{{ s.label }}</span>
      </label>
      <label class="pick">
        <input type="checkbox" :checked="accessibility.dyslexiaFont" data-testid="dyslexia-font" @change="setAccessibility({ dyslexiaFont: checked($event) })" />
        <span>Dyslexia-friendly font <small>Sets every text in OpenDyslexic.</small></span>
      </label>
    </fieldset>
    <fieldset class="g-card stack">
      <legend>Motion and touch</legend>
      <label class="pick">
        <input type="checkbox" :checked="accessibility.reduceMotion" data-testid="reduce-motion" @change="setAccessibility({ reduceMotion: checked($event) })" />
        <span>Reduce motion <small>Dice land flat, and nothing slides, tumbles or pulses.</small></span>
      </label>
      <p v-if="deviceReducesMotion()" class="hint" data-testid="device-reduces-motion">Your device already asks for reduced motion, so motion is reduced whatever is chosen here.</p>
      <label class="pick">
        <input type="checkbox" :checked="accessibility.haptics" data-testid="haptics" @change="haptics(checked($event))" />
        <span>Haptics <small>A buzz when your turn starts and when your roll lands.</small></span>
      </label>
      <p v-if="!canBuzz()" class="hint" data-testid="no-haptics">This device cannot vibrate.</p>
    </fieldset>
    <fieldset class="g-card stack">
      <legend>Screen reader</legend>
      <label class="pick">
        <input type="checkbox" :checked="accessibility.announceTurns" data-testid="announce-turns" @change="setAccessibility({ announceTurns: checked($event) })" />
        <span>Announce turns <small>Whose turn starts, and when it is yours.</small></span>
      </label>
      <label class="pick">
        <input type="checkbox" :checked="accessibility.announceRolls" data-testid="announce-rolls" @change="setAccessibility({ announceRolls: checked($event) })" />
        <span>Announce rolls <small>Who rolled, what for and the total.</small></span>
      </label>
    </fieldset>
    <div>
      <GButton type="button" variant="secondary" data-testid="reset-accessibility" @click="resetAccessibility">Back to the standard look</GButton>
    </div>
  </main>
</template>

<style scoped>
.stack {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 0;
}
legend {
  padding: 0 6px;
  font-family: var(--font-display);
  font-size: 18px;
  color: var(--color-gold-high);
}
.pick {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: var(--size-touch-target);
  font-size: 17px;
}
.pick input {
  width: 20px;
  height: 20px;
  accent-color: var(--color-gold);
}
.pick small {
  display: block;
  font-size: 14px;
  color: var(--color-text-2);
}
.hint {
  margin: 0;
  font-size: 14px;
  color: var(--color-text-2);
}
.sample {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 6px 0 0;
}
.chip {
  padding: 4px 12px;
  border: 1px solid currentcolor;
  border-radius: var(--radius-chip);
  font-size: 14px;
}
.chip--party {
  color: var(--color-party);
  background: var(--color-party-fill);
}
.chip--enemy {
  color: var(--color-enemy);
  background: var(--color-enemy-fill);
}
.chip--success {
  color: var(--color-success);
}
</style>

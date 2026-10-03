<script setup lang="ts">
import { computed } from 'vue'
import type { DieLook } from '@/infrastructure/api/types.gen'
import { SHEET_FACES, cellOf, pictureStyle, sheetCols, type DieType } from './sets'

// The unwrapped faces of one die: its pattern and colours, the set's picture where it was placed, and
// the numbers on their own layer above both. The 3D dice are painted from the same sheet.
const props = withDefaults(defineProps<{ type: DieType; look: DieLook; imageUrl?: string; size?: number }>(), { imageUrl: undefined, size: 220 })
const cols = computed(() => sheetCols(SHEET_FACES[props.type]))
const cells = computed(() => Array.from({ length: SHEET_FACES[props.type] }, (_, i) => ({ n: i + 1, ...cellOf(i, cols.value) })))
const pictured = computed(() => Boolean(props.imageUrl && props.look.image))
const label = computed(() => `The unwrapped faces of the ${props.type}: ${props.look.pattern}${pictured.value ? ', with the picture on it' : ''}`)
</script>

<template>
  <div :class="['sheet', `sheet--${look.pattern}`]" :style="{ '--body': look.body, width: `${size}px` }" role="img" :aria-label="label" data-testid="dice-sheet">
    <img v-if="look.image && imageUrl" :src="imageUrl" alt="" class="picture" :style="pictureStyle(look.image)" data-testid="sheet-picture" />
    <svg :viewBox="`0 0 ${cols} ${cols}`" class="faces" aria-hidden="true">
      <g v-for="c in cells" :key="c.n">
        <rect :x="c.col + 0.05" :y="c.row + 0.05" width="0.9" height="0.9" rx="0.08" fill="none" :stroke="look.numbers" stroke-width="0.025" />
        <text :x="c.col + 0.5" :y="c.row + 0.66" text-anchor="middle" font-family="Cinzel" font-weight="700" font-size="0.44" :fill="look.numbers">{{ c.n }}</text>
      </g>
    </svg>
  </div>
</template>

<style scoped>
.sheet {
  position: relative;
  overflow: hidden;
  aspect-ratio: 1;
  max-width: 100%;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--body);
}
.sheet--marble {
  background:
    radial-gradient(ellipse at 20% 30%, color-mix(in srgb, var(--body), white 35%) 0 8%, transparent 30%),
    radial-gradient(ellipse at 72% 64%, color-mix(in srgb, var(--body), white 22%) 0 6%, transparent 28%),
    linear-gradient(115deg, transparent 40%, color-mix(in srgb, var(--body), white 28%) 47%, transparent 54%),
    var(--body);
}
.sheet--speckled {
  background:
    radial-gradient(color-mix(in srgb, var(--body), white 45%) 12%, transparent 14%) 0 0 / 14px 14px,
    radial-gradient(color-mix(in srgb, var(--body), black 40%) 10%, transparent 12%) 7px 7px / 14px 14px,
    var(--body);
}
.sheet--stripes {
  background: repeating-linear-gradient(45deg, var(--body) 0 10px, color-mix(in srgb, var(--body), white 30%) 10px 20px);
}
.picture {
  position: absolute;
  max-width: none;
}
.faces {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}
</style>

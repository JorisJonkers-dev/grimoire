<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{ sides: 4 | 6 | 8 | 10 | 12 | 20 | 100; value?: number | null; state?: 'idle' | 'rolling' | 'kept' | 'dropped'; size?: number }>(),
  { value: null, state: 'idle', size: 84 },
)
const face = computed(() => (props.value === null ? '?' : String(props.value)))
const label = computed(() => {
  const shown = props.value === null ? 'not rolled yet' : `showing ${props.value}`
  const state = props.state === 'idle' ? '' : `, ${props.state}`
  return `d${props.sides} ${shown}${state}`
})
</script>

<template>
  <span :class="['die', `die--${state}`]" role="img" :aria-label="label">
    <svg :width="size" :height="size" viewBox="0 0 100 100" aria-hidden="true">
      <template v-if="sides === 20">
        <polygon points="50,4 10,27 50,30" fill="#E0685A" />
        <polygon points="50,4 90,27 50,30" fill="#C4473B" />
        <polygon points="90,27 73,66 50,30" fill="#A8362C" />
        <polygon points="10,27 27,66 50,30" fill="#D45547" />
        <polygon points="90,27 90,73 73,66" fill="#7E231C" />
        <polygon points="10,27 10,73 27,66" fill="#B8402F" />
        <polygon points="90,73 50,96 73,66" fill="#5E1712" />
        <polygon points="10,73 50,96 27,66" fill="#8C2A20" />
        <polygon points="27,66 73,66 50,96" fill="#76201A" />
        <polygon points="50,30 73,66 27,66" fill="#EF7B6B" />
        <polygon points="50,4 90,27 90,73 50,96 10,73 10,27" fill="none" stroke="#3A0E0A" stroke-width="2" />
        <text x="50" y="59" text-anchor="middle" font-family="Cinzel" font-weight="700" font-size="19" fill="#F8E7B0">{{ face }}</text>
      </template>
      <template v-else-if="sides === 4">
        <polygon points="50,6 6,88 50,64" fill="#F1E3B8" />
        <polygon points="50,6 94,88 50,64" fill="#D2B86C" />
        <polygon points="6,88 94,88 50,64" fill="#A88A3E" />
        <polygon points="50,6 94,88 6,88" fill="none" stroke="#4A3810" stroke-width="2" />
        <text x="36" y="62" text-anchor="middle" font-family="Cinzel" font-weight="700" font-size="18" fill="#2B2118">{{ face }}</text>
      </template>
      <template v-else-if="sides === 6">
        <polygon points="50,8 90,28 50,48 10,28" fill="#F4ECDA" />
        <polygon points="10,28 50,48 50,94 10,74" fill="#D9C9A3" />
        <polygon points="90,28 50,48 50,94 90,74" fill="#B8A57C" />
        <polygon points="50,8 90,28 90,74 50,94 10,74 10,28" fill="none" stroke="#2B2118" stroke-width="2" />
        <text x="50" y="35" text-anchor="middle" font-family="Cinzel" font-weight="700" font-size="18" fill="#2B2118">{{ face }}</text>
      </template>
      <template v-else-if="sides === 10 || sides === 100">
        <polygon points="50,4 92,44 50,56" fill="#B7D9A8" />
        <polygon points="50,4 8,44 50,56" fill="#D2E8C6" />
        <polygon points="8,44 50,96 50,56" fill="#8FBF7C" />
        <polygon points="92,44 50,96 50,56" fill="#6E9E5C" />
        <polygon points="50,4 92,44 50,96 8,44" fill="none" stroke="#1D2B23" stroke-width="2" />
        <text x="50" y="44" text-anchor="middle" font-family="Cinzel" font-weight="700" font-size="16" fill="#1D2B23">{{ face }}</text>
      </template>
      <template v-else-if="sides === 12">
        <polygon points="50,6 93,37 77,88 23,88 7,37" fill="#D9B8E8" />
        <polygon points="50,24 72,40 64,68 36,68 28,40" fill="#C49BD8" />
        <polygon points="50,6 93,37 77,88 23,88 7,37" fill="none" stroke="#2E1A38" stroke-width="2" />
        <text x="50" y="56" text-anchor="middle" font-family="Cinzel" font-weight="700" font-size="17" fill="#2E1A38">{{ face }}</text>
      </template>
      <template v-else>
        <polygon points="50,4 94,50 50,58" fill="#9EC6E8" />
        <polygon points="50,4 6,50 50,58" fill="#C3DDF2" />
        <polygon points="6,50 50,96 50,58" fill="#6F9CC4" />
        <polygon points="94,50 50,96 50,58" fill="#4F7BA3" />
        <polygon points="50,4 94,50 50,96 6,50" fill="none" stroke="#1A2430" stroke-width="2" />
        <text x="40" y="44" text-anchor="middle" font-family="Cinzel" font-weight="700" font-size="17" fill="#1A2430">{{ face }}</text>
      </template>
    </svg>
  </span>
</template>

<style scoped>
.die {
  display: inline-block;
  transform-origin: 50% 55%;
}
.die--rolling {
  animation: tumble var(--motion-dice-tumble) cubic-bezier(0.2, 0.7, 0.3, 1) both;
}
.die--kept {
  filter: drop-shadow(0 0 10px rgb(235 203 122 / 90%));
}
.die--dropped {
  opacity: 0.4;
  filter: grayscale(0.5);
}
@keyframes tumble {
  0% { transform: translateY(-46px) rotate(0deg) scale(0.78); }
  28% { transform: translateY(8px) rotate(250deg) scale(1.06); }
  52% { transform: translateY(-14px) rotate(470deg) scale(0.94); }
  78% { transform: translateY(3px) rotate(655deg) scale(1.02); }
  100% { transform: translateY(0) rotate(720deg) scale(1); }
}
</style>

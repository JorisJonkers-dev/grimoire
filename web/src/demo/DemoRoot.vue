<script setup lang="ts">
import { useQueryClient } from '@tanstack/vue-query'
import { ref } from 'vue'
import App from '@/app/App.vue'
import DemoRoleSwitcher from './DemoRoleSwitcher.vue'
import type { Persona } from './role'
import type { World } from './world'

const props = defineProps<{ world: World }>()
const client = useQueryClient()
// Switching who is looking throws away every answer the old caller got and draws the app anew, as
// signing in as someone else would.
const generation = ref(0)
function switchTo(persona: Persona) {
  // The world is reactive and owned by the demo: the switcher is its one writer besides sign-in and sign-out.
  // eslint-disable-next-line vue/no-mutating-props
  props.world.persona = persona
  client.clear()
  generation.value += 1
}
</script>

<template>
  <App :key="generation" />
  <DemoRoleSwitcher :persona="world.persona" @switch="switchTo" />
</template>

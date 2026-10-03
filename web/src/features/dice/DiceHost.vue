<script setup lang="ts">
import DiceStage from './DiceStage.vue'
import { useDiceStage } from './stage'

// The page's dice stage, showing whatever was last thrown in this tab.
const stage = useDiceStage()
</script>

<template>
  <DiceStage :roll="stage.roll.value" :n="stage.n.value" />
</template>

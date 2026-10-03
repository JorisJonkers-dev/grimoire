<script setup lang="ts">
import { computed } from 'vue'
import DiceStage from './DiceStage.vue'
import { useDiceStage } from './stage'
import { useMyDice } from './useMyDice'

// The page's dice stage, showing whatever was last thrown in this tab. The viewer's own roll is thrown
// in the Dice Set they chose, picture and all; anyone else's comes with the look it may show.
const stage = useDiceStage()
const mine = useMyDice()
const roll = computed(() => (stage.roll.value && stage.own.value && mine.value ? { ...stage.roll.value, look: mine.value } : stage.roll.value))
</script>

<template>
  <DiceStage :roll="roll" :n="stage.n.value" />
</template>

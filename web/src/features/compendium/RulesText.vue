<script setup lang="ts">
import { computed, ref } from 'vue'
import type { ConditionRef } from '@/infrastructure/api/types.gen'
import { highlight } from './highlight'

const props = defineProps<{ text: string; mentions: readonly ConditionRef[] }>()
const open = ref<ConditionRef | null>(null)
const names = computed(() => props.mentions.map((m) => m.name))
const paragraphs = (text: string) => text.split(/\n+/).filter((p) => p.trim() !== '')
const byName = (name: string) => props.mentions.find((m) => m.name === name) ?? null
</script>

<template>
  <div class="rules-text">
    <p v-for="(para, i) in paragraphs(text)" :key="i">
      <template v-for="(seg, j) in highlight(para, names)" :key="j">
        <button v-if="seg.mention" type="button" class="term" :aria-expanded="open?.name === seg.mention" @click="open = byName(seg.mention)">{{ seg.text }}</button>
        <template v-else>{{ seg.text }}</template>
      </template>
    </p>
    <section v-if="open" class="popover" role="dialog" :aria-label="open.name" data-testid="condition-popover">
      <h3>{{ open.name }}</h3>
      <p v-for="(para, i) in paragraphs(open.description)" :key="i">{{ para }}</p>
      <button type="button" class="close" @click="open = null">Close</button>
    </section>
  </div>
</template>

<style scoped>
.rules-text {
  font-family: var(--font-flavour);
  font-size: 18px;
  line-height: 1.5;
}
.term {
  padding: 0 2px;
  border: 0;
  border-bottom: 1px dashed var(--color-gold-high);
  background: none;
  color: var(--color-gold-high);
  font: inherit;
  cursor: pointer;
}
.popover {
  padding: 14px 16px;
  border: 1px solid var(--color-gold);
  border-radius: var(--radius-lg);
  background: var(--color-raised);
  font-family: var(--font-body);
  font-size: 16px;
}
.popover h3 {
  margin: 0 0 6px;
  font-family: var(--font-display);
  font-size: 16px;
}
.close {
  min-height: 44px;
  padding: 0 16px;
  border: 1px solid var(--color-bronze);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  color: var(--color-text);
}
</style>

<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed } from 'vue'
import { getAccountOptions, getUnseenReleaseNoteOptions, getUnseenReleaseNoteQueryKey, seeReleaseNoteMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'

const client = useQueryClient()
const account = useQuery(getAccountOptions())
const unseen = useQuery(computed(() => ({ ...getUnseenReleaseNoteOptions(), enabled: !!account.data.value })))
const note = computed(() => unseen.data.value?.note)
const lines = computed(() => (note.value?.body ?? '').split('\n').map((l) => l.trim()).filter(Boolean))
// A body of dashed lines reads as a list; anything else as paragraphs.
const bulleted = computed(() => lines.value.some((l) => l.startsWith('- ')))
const see = useMutation(seeReleaseNoteMutation())
function dismiss() {
  if (!note.value) return
  see.mutate({ path: { noteId: note.value.id } }, { onSuccess: () => void client.invalidateQueries({ queryKey: getUnseenReleaseNoteQueryKey() }) })
}
</script>

<template>
  <section v-if="note" class="g-card release" aria-labelledby="release-title" data-testid="release-note">
    <p class="eyebrow">New in {{ note.version }}</p>
    <h2 id="release-title">{{ note.title }}</h2>
    <ul v-if="bulleted">
      <li v-for="(l, i) in lines" :key="i">{{ l.replace(/^- /, '') }}</li>
    </ul>
    <template v-else>
      <p v-for="(l, i) in lines" :key="i">{{ l }}</p>
    </template>
    <button type="button" class="dismiss" :disabled="see.isPending.value" data-testid="release-note-seen" @click="dismiss">Got it</button>
  </section>
</template>

<style scoped>
.release {
  display: flex;
  flex-direction: column;
  gap: 8px;
  border-color: var(--color-gold);
}
.eyebrow {
  margin: 0;
  color: var(--color-gold);
  font-size: 13px;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}
h2 {
  margin: 0;
  font-size: 20px;
}
ul,
p {
  margin: 0;
}
.dismiss {
  align-self: flex-start;
  padding: 6px 14px;
  border: 1px solid var(--color-gold);
  border-radius: 8px;
  background: none;
  color: var(--color-gold-high);
  cursor: pointer;
}
</style>

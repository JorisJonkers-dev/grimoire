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
  <section v-if="note" class="release" aria-labelledby="release-title" data-testid="release-note">
    <p class="eyebrow">New in {{ note.version }}</p>
    <h2 id="release-title">{{ note.title }}</h2>
    <ul v-if="bulleted">
      <li v-for="(l, i) in lines" :key="i">{{ l.replace(/^- /, '') }}</li>
    </ul>
    <template v-else>
      <p v-for="(l, i) in lines" :key="i">{{ l }}</p>
    </template>
    <button type="button" class="dismiss" :disabled="see.isPending.value" data-testid="release-note-seen" @click="dismiss">Dismiss</button>
  </section>
</template>

<style scoped>
/* What is new stands under a rule, small, at the side: it is news, not the page. */
.release {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-top: 18px;
  border-top: 1px solid var(--color-rule-soft);
  font-size: 15px;
  line-height: 1.45;
}
.eyebrow {
  margin: 0;
  font-size: 14px;
  color: var(--color-text-3);
}
h2 {
  margin: 0;
  font-weight: 700;
  letter-spacing: 0;
  text-transform: none;
  color: var(--color-text);
}
ul,
p {
  margin: 0;
}
ul {
  padding-left: 18px;
}
.dismiss {
  align-self: flex-start;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--color-text-3);
  font-family: var(--font-ui);
  font-size: 14px;
  text-decoration: underline;
  text-underline-offset: 3px;
  cursor: pointer;
}
.dismiss:hover {
  color: var(--color-gold-high);
}
</style>

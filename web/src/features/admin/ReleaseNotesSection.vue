<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import {
  draftReleaseNoteMutation,
  editReleaseNoteMutation,
  listReleaseNotesOptions,
  listReleaseNotesQueryKey,
  publishReleaseNoteMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { ReleaseNote } from '@/infrastructure/api/types.gen'
import { GButton, GField } from '@/shared/ui'
import { when } from './labels'

const client = useQueryClient()
const list = useQuery(listReleaseNotesOptions())
const refresh = () => client.invalidateQueries({ queryKey: listReleaseNotesQueryKey() })
const draft = useMutation(draftReleaseNoteMutation())
const edit = useMutation(editReleaseNoteMutation())
const publish = useMutation(publishReleaseNoteMutation())
const version = ref('')
const editing = ref<ReleaseNote>()
const title = ref('')
const body = ref('')
const at = ref('')
const failed = computed(() => [draft, edit, publish].find((m) => m.isError.value)?.error.value as { status?: number } | null | undefined)

function open(n: ReleaseNote) {
  editing.value = n
  title.value = n.title
  body.value = n.body
  at.value = ''
}
function start() {
  draft.mutate({ body: { version: version.value.trim() } }, { onSuccess: (n) => {
      version.value = ''
      open(n)
      void refresh()
    } })
}
function save() {
  const n = editing.value
  if (!n) return
  edit.mutate({ path: { noteId: n.id }, body: { title: title.value.trim(), body: body.value } }, { onSuccess: (saved) => {
      open(saved)
      void refresh()
    } })
}
function release() {
  const n = editing.value
  if (!n) return
  publish.mutate({ path: { noteId: n.id }, body: at.value ? { at: new Date(at.value).toISOString() } : {} }, { onSuccess: () => {
      editing.value = undefined
      void refresh()
    } })
}
</script>

<template>
  <section class="g-card stack" data-testid="release-notes">
    <h2>Release Notes</h2>
    <ul v-if="list.data.value?.items.length" class="rows">
      <li v-for="n in list.data.value.items" :key="n.id" :data-testid="`release-${n.version}`">
        <span><strong>{{ n.version }}</strong> {{ n.title }}</span>
        <span class="meta">
          <span :class="['chip', n.status]">{{ n.status === 'draft' ? 'Draft' : n.status === 'scheduled' ? `Scheduled ${when(n.publishAt ?? '')}` : 'Published' }}</span>
          <button v-if="n.status !== 'published'" type="button" class="link" :data-testid="`release-open-${n.version}`" @click="open(n)">Edit</button>
        </span>
      </li>
    </ul>
    <form class="row" data-testid="release-draft" @submit.prevent="start">
      <GField v-model="version" label="Full release" :maxlength="21" hint="Such as 1.2.0; its features come from the changelog." data-testid="release-version" />
      <GButton type="submit" :disabled="!version.trim() || draft.isPending.value">Draft the Release Note</GButton>
    </form>
    <form v-if="editing" class="stack" data-testid="release-editor" @submit.prevent="save">
      <GField v-model="title" label="Title" :maxlength="120" required data-testid="release-title" />
      <GField v-model="body" label="What is new" multiline :maxlength="8000" hint="One line per change, each starting with a dash." data-testid="release-body" />
      <label class="g-field">
        <span>Publish at (leave empty for now)</span>
        <input v-model="at" type="datetime-local" data-testid="release-at" />
      </label>
      <div class="row">
        <GButton type="submit" :disabled="!title.trim() || edit.isPending.value">Save</GButton>
        <GButton type="button" variant="primary" :disabled="publish.isPending.value" data-testid="release-publish" @click="release">{{ at ? 'Schedule' : 'Publish now' }}</GButton>
      </div>
    </form>
    <p v-if="failed" role="alert" class="g-alert" data-testid="release-failed">
      {{ failed.status === 409 ? 'That release already has its Release Note, or this one is already out.' : failed.status === 422 ? 'Use a full release such as 1.2.0 and a title.' : 'That did not work. Try again shortly.' }}
    </p>
  </section>
</template>

<style scoped>
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
h2 {
  margin: 0;
  font-size: 18px;
}
.rows {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  list-style: none;
}
.rows li {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 0;
  border-bottom: 1px solid var(--color-line);
}
.meta {
  display: flex;
  align-items: center;
  gap: 8px;
}
.chip {
  padding: 2px 8px;
  border: 1px solid var(--color-line);
  border-radius: 999px;
  font-size: 13px;
}
.chip.published {
  border-color: var(--color-gold);
}
.link {
  border: 0;
  background: none;
  color: var(--color-gold);
  cursor: pointer;
  text-decoration: underline;
}
</style>

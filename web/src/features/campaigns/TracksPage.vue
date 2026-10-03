<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { adjustTrackMutation, createTrackMutation, deleteTrackMutation, listRuleHooksOptions, listTracksOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Track, TrackScope, TrackStanding, TrackThreshold } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

// A Campaign's Tracks: scores such as stress or renown, kept for each Character or for the party. The
// DM keeps them, moves the scores and sees the thresholds; a Player sees where they and the party stand.
const route = useRoute()
const client = useQueryClient()
const campaignId = String(route.params.id)
const path = { path: { campaignId } }
const list = useQuery({ ...listTracksOptions(path), retry: false })
const dm = computed(() => list.data.value?.dm ?? false)
const tracks = computed(() => list.data.value?.tracks ?? [])
// The Roll Tables the Campaign sees, for naming and choosing what a threshold rolls on.
const own = useQuery(computed(() => ({ ...listRuleHooksOptions(path), enabled: dm.value, retry: false })))
const tables = computed(() => own.data.value?.tables ?? [])

const kept = (t: Track) => `Kept for ${t.scope === 'party' ? 'the party' : 'each Character'}, from ${String(t.min)} to ${String(t.max)}.`
const who = (st: TrackStanding) => st.name || 'The party'
function says(th: TrackThreshold): string {
  const line = `At ${String(th.at)} on the way ${th.rising ? 'up' : 'down'}: ${th.label}.`
  if (th.rollTableId) return `${line} Rolls on ${tables.value.find((t) => t.id === th.rollTableId)?.name ?? 'a Roll Table this Campaign no longer sees'}.`
  return th.effect ? `${line} Applies ${th.effect}.` : line
}

const problem = ref('')
const moved = ref('')
const failed = () => {
  moved.value = ''
  problem.value = 'That could not be done. Check what you entered and try again.'
}
const done = (news = '') => {
  problem.value = ''
  moved.value = news
  void client.invalidateQueries()
}

// How far one press moves a Track's score.
const by = reactive<Record<string, number>>({})
const stepOf = (t: Track) => by[t.id] ?? 1
const adjust = useMutation(adjustTrackMutation())
function move(t: Track, st: TrackStanding, sign: 1 | -1) {
  const body = { delta: sign * stepOf(t), characterId: st.characterId }
  adjust.mutate({ path: { campaignId, trackId: t.id }, body }, {
    onSuccess: (r) => {
      const crossed = r.crossed.length > 0 ? ` Crossed: ${r.crossed.map((th) => th.label).join(', ')}.` : ''
      done(`${t.name} for ${st.name || 'the party'} is now ${String(r.value)}.${crossed}`)
    },
    onError: failed,
  })
}
const drop = useMutation(deleteTrackMutation())
const remove = (t: Track) => { drop.mutate({ path: { campaignId, trackId: t.id } }, { onSuccess: () => { done() }, onError: failed }) }

type ThresholdDraft = { at: number; way: 'up' | 'down'; label: string; effect: string; table: string }
const fresh = () => ({ name: '', scope: 'character' as TrackScope, min: 0, max: 10, start: 0, thresholds: [] as ThresholdDraft[] })
const draft = reactive(fresh())
const create = useMutation(createTrackMutation())
function add() {
  const thresholds = draft.thresholds.map((th) => ({
    at: th.at, rising: th.way === 'up', label: th.label.trim(),
    ...(th.table ? { rollTableId: th.table } : th.effect.trim() ? { effect: th.effect.trim() } : {}),
  }))
  create.mutate({ ...path, body: { name: draft.name.trim(), scope: draft.scope, min: draft.min, max: draft.max, start: draft.start, thresholds } }, {
    onSuccess: () => {
      Object.assign(draft, fresh())
      done()
    },
    onError: failed,
  })
}
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'campaign', params: { id: campaignId } }" class="back">← Campaign</RouterLink>
    <h1>Tracks</h1>
    <p v-if="list.isError.value" role="alert" class="g-alert" data-testid="tracks-missing">That Campaign is not available.</p>
    <template v-else-if="list.isSuccess.value">
      <p class="hint">Scores this Campaign keeps, such as stress, honour or renown.</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="tracks-problem">{{ problem }}</p>
      <p v-if="moved" role="status" class="moved" data-testid="tracks-moved">{{ moved }}</p>
      <p v-if="tracks.length === 0" class="hint" data-testid="no-tracks">No Tracks in this Campaign yet.</p>
      <section v-for="t in tracks" :key="t.id" class="g-card track" :aria-labelledby="`track-title-${t.id}`" :data-testid="`track-${t.id}`">
        <h2 :id="`track-title-${t.id}`">{{ t.name }}</h2>
        <p class="hint" data-testid="track-kept">{{ kept(t) }}</p>
        <label v-if="dm" class="g-field small"><span>Move by</span><input v-model.number="by[t.id]" type="number" min="1" max="2000" placeholder="1" data-testid="by" /></label>
        <ul class="g-list">
          <li v-for="st in t.standings" :key="st.characterId ?? 'party'" class="row">
            <span data-testid="standing">{{ who(st) }}: {{ st.value }}</span>
            <template v-if="dm">
              <GButton :aria-label="`Lower ${t.name} for ${st.name || 'the party'}`" data-testid="lower" @click="move(t, st, -1)">−</GButton>
              <GButton :aria-label="`Raise ${t.name} for ${st.name || 'the party'}`" data-testid="raise" @click="move(t, st, 1)">+</GButton>
            </template>
          </li>
        </ul>
        <template v-if="dm">
          <ul v-if="t.thresholds?.length" class="g-list" :aria-label="`Thresholds of ${t.name}`" data-testid="thresholds">
            <li v-for="(th, i) in t.thresholds" :key="i">{{ says(th) }}</li>
          </ul>
          <GButton variant="danger" :aria-label="`Remove ${t.name}`" data-testid="track-remove" @click="remove(t)">Remove</GButton>
        </template>
      </section>

      <form v-if="dm" class="g-card track" aria-label="Add a Track" data-testid="track-add" @submit.prevent="add()">
        <h2>Add a Track</h2>
        <label class="g-field"><span>Name</span><input v-model="draft.name" maxlength="80" data-testid="track-name" /></label>
        <label class="g-field">
          <span>Kept for</span>
          <select v-model="draft.scope" data-testid="track-scope">
            <option value="character">Each Character</option>
            <option value="party">The party</option>
          </select>
        </label>
        <div class="row">
          <label class="g-field small"><span>Lowest</span><input v-model.number="draft.min" type="number" min="-1000" max="1000" data-testid="track-min" /></label>
          <label class="g-field small"><span>Highest</span><input v-model.number="draft.max" type="number" min="-1000" max="1000" data-testid="track-max" /></label>
          <label class="g-field small"><span>Starts at</span><input v-model.number="draft.start" type="number" min="-1000" max="1000" data-testid="track-start" /></label>
        </div>
        <h3>Thresholds</h3>
        <ol class="rows">
          <li v-for="(th, i) in draft.thresholds" :key="i" class="row">
            <label class="g-field small"><span>At</span><input v-model.number="th.at" type="number" min="-1000" max="1000" :data-testid="`threshold-${String(i)}-at`" /></label>
            <label class="g-field">
              <span>On the way</span>
              <select v-model="th.way" :data-testid="`threshold-${String(i)}-way`">
                <option value="up">Up</option>
                <option value="down">Down</option>
              </select>
            </label>
            <label class="g-field grow"><span>What it is called</span><input v-model="th.label" maxlength="80" :data-testid="`threshold-${String(i)}-label`" /></label>
            <label v-if="tables.length > 0" class="g-field">
              <span>Rolls on</span>
              <select v-model="th.table" :data-testid="`threshold-${String(i)}-table`">
                <option value="">No table</option>
                <option v-for="rt in tables" :key="rt.id" :value="rt.id">{{ rt.name }}</option>
              </select>
            </label>
            <label v-if="th.table === ''" class="g-field"><span>Effect it applies, by slug</span><input v-model="th.effect" maxlength="80" :data-testid="`threshold-${String(i)}-effect`" /></label>
            <GButton :aria-label="`Remove threshold ${String(i + 1)}`" :data-testid="`threshold-${String(i)}-remove`" @click="draft.thresholds.splice(i, 1)">×</GButton>
          </li>
        </ol>
        <GButton data-testid="threshold-add" @click="draft.thresholds.push({ at: 0, way: 'up', label: '', effect: '', table: '' })">+ Threshold</GButton>
        <GButton type="submit" variant="primary" :disabled="draft.name.trim() === ''">Add Track</GButton>
      </form>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
.track {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.track h2,
.track h3 {
  margin: 0;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.small {
  max-width: 120px;
}
.grow {
  flex: 1;
  min-width: 160px;
}
.moved {
  margin: 0;
  color: var(--color-gold-high);
}
</style>

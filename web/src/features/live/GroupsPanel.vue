<script setup lang="ts">
import { computed, ref } from 'vue'
import type { LiveGroup, LiveToken, LocalMap } from '@/infrastructure/api/types.gen'
import { GButton, GField } from '@/shared/ui'

// A split party plays as groups, each in a Session of its own with its own map, fog and fight. The DM
// sends party tokens off as a group, goes from group to group, chooses the one the Table Display
// follows, and brings a group back to the Session the party split from.
type Command =
  | { kind: 'split_party'; name: string; tokenIds: string[]; mapId: string; q: number; r: number }
  | { kind: 'rejoin_party'; sessionId: string; q: number; r: number }
  | { kind: 'table_follow'; sessionId?: string }
const props = defineProps<{ campaignId: string; groups: LiveGroup[]; tokens: LiveToken[]; maps: LocalMap[] }>()
const emit = defineEmits<{ send: [command: Command] }>()

const name = ref('')
const mapId = ref('')
const going = ref<string[]>([])
const party = computed(() => props.tokens.filter((t) => t.kind === 'party'))
const localMaps = computed(() => props.maps.filter((m) => m.kind === 'local'))
const here = computed(() => props.groups.find((g) => g.here))
/** A group that left the party runs its own Session; splitting, following and returning are done from the party's. */
const away = computed(() => Boolean(here.value) && !here.value?.home)
const ready = computed(() => name.value.trim() !== '' && mapId.value !== '' && going.value.length > 0)
const title = (g: LiveGroup) => (g.home ? 'The party' : g.name)

function split() {
  if (!ready.value) return
  emit('send', { kind: 'split_party', name: name.value.trim(), tokenIds: going.value, mapId: mapId.value, q: 0, r: 0 })
  name.value = ''
  going.value = []
}
// A group comes back beside a party token that stayed, or at the middle of the map when none did.
function back(g: LiveGroup) {
  const beside = party.value[0]
  emit('send', { kind: 'rejoin_party', sessionId: g.sessionId, q: beside?.q ?? 0, r: beside?.r ?? 0 })
}
const follow = (g: LiveGroup) => { emit('send', g.home ? { kind: 'table_follow' } : { kind: 'table_follow', sessionId: g.sessionId }) }
</script>

<template>
  <section class="g-card panel" aria-label="Party groups" data-testid="groups">
    <h2>Party groups</h2>
    <ul v-if="groups.length" class="g-list">
      <li v-for="g in groups" :key="g.sessionId" class="row" :data-testid="`group-${g.sessionId}`">
        <span>
          <strong>{{ title(g) }}</strong>
          <span class="dim"> {{ g.tokens.join(', ') }}</span>
        </span>
        <span class="actions">
          <span v-if="g.here" class="g-tag">You are here</span>
          <RouterLink v-else :to="{ name: 'session', params: { id: campaignId, sid: g.sessionId } }" :data-testid="`group-go-${g.sessionId}`">Go to this group</RouterLink>
          <label v-if="!away" class="check">
            <input type="radio" name="table-follows" :checked="g.table" :data-testid="`group-table-${g.sessionId}`" @change="follow(g)" />
            <span>Table Display follows</span>
          </label>
          <GButton v-if="!away && !g.home" :aria-label="`Bring ${g.name} back`" :data-testid="`group-back-${g.sessionId}`" @click="back(g)">Bring back</GButton>
        </span>
      </li>
    </ul>
    <p v-if="groups.length" class="dim" data-testid="groups-table">
      The Table Display shows the group it follows. While the party is split it opens for you, and for the Players of that group.
    </p>
    <p v-if="away" class="dim" data-testid="groups-away">This group left the party. Bring it back, or send another group off, from the party's own Session.</p>
    <form v-else class="split" data-testid="group-form" @submit.prevent="split">
      <h3>Send a group off</h3>
      <GField v-model="name" label="Name the group" :maxlength="40" data-testid="group-name" />
      <fieldset class="who">
        <legend>Who goes</legend>
        <label v-for="t in party" :key="t.id" class="check">
          <input v-model="going" type="checkbox" :value="t.id" :data-testid="`goes-${t.label}`" />
          <span>{{ t.label }}</span>
        </label>
        <p v-if="party.length === 0" class="dim">There are no party tokens here.</p>
      </fieldset>
      <label class="g-field">
        <span>To the map</span>
        <select v-model="mapId" data-testid="group-map">
          <option value="">Choose a map</option>
          <option v-for="m in localMaps" :key="m.id" :value="m.id">{{ m.name }}</option>
        </select>
      </label>
      <GButton type="submit" :disabled="!ready" data-testid="group-split">Split the party</GButton>
      <p class="dim">The group plays on in a Session of its own, between scenes only. Its Players are taken there; nobody who stays sees where it went.</p>
    </form>
  </section>
</template>

<style scoped>
.panel,
.split {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
h2,
h3,
p {
  margin: 0;
}
h3 {
  font-size: 16px;
}
.row,
.actions,
.who {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.row {
  justify-content: space-between;
}
.who {
  margin: 0;
  padding: 8px 10px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
}
.check {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 44px;
}
.check input {
  width: 20px;
  height: 20px;
}
.dim {
  color: var(--color-text-2);
}
</style>

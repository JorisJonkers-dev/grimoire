<script setup lang="ts">
import CampaignFrame from '@/features/campaigns/CampaignFrame.vue'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { listMapsOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { uploadMap, useDefaultWorld } from '@/infrastructure/api/sdk.gen'
import type { MapKind } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const campaignId = String(route.params.id)
const maps = useQuery({ ...listMapsOptions({ path: { campaignId } }), retry: false })
const name = ref('')
const kind = ref<MapKind>('local')
const file = ref<File | null>(null)
const failed = ref('')
const busy = ref(false)
const ready = computed(() => name.value.trim() !== '' && file.value !== null && !busy.value)

function pick(event: Event) {
  file.value = (event.target as HTMLInputElement).files?.[0] ?? null
}
async function upload() {
  const picture = file.value
  if (!picture) return
  busy.value = true
  failed.value = ''
  try {
    const body = (await picture.arrayBuffer()) as unknown as Blob
    const { data } = await uploadMap({ path: { campaignId }, query: { name: name.value.trim(), kind: kind.value }, body, requestValidator: undefined, throwOnError: true })
    void client.invalidateQueries()
    void router.push({ name: 'map', params: { id: campaignId, mapId: data.id } })
  } catch {
    failed.value = 'The map could not be uploaded. Use a PNG, JPEG or WebP of at most 25 MB.'
  } finally {
    busy.value = false
  }
}
const worldFailed = ref(false)
async function takeDefaultWorld() {
  busy.value = true
  worldFailed.value = false
  try {
    const { data } = await useDefaultWorld({ path: { campaignId }, throwOnError: true })
    void client.invalidateQueries()
    void router.push({ name: 'map', params: { id: campaignId, mapId: data.id } })
  } catch {
    worldFailed.value = true
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <main class="g-page">
    <CampaignFrame current="maps">
      <header class="g-headline">
        <span class="g-eyebrow">Campaign prep</span>
        <h1>Maps</h1>
      </header>
    </CampaignFrame>
    <p v-if="maps.isError.value" role="alert" class="g-alert" data-testid="maps-refused">Only the DM can manage maps.</p>
    <template v-else>
      <ul class="g-list" data-testid="map-list">
        <li v-for="m in maps.data.value ?? []" :key="m.id">
          <RouterLink :to="{ name: 'map', params: { id: campaignId, mapId: m.id } }">{{ m.name }}</RouterLink>
          <span class="hint"> · {{ m.kind === 'world' ? 'world map' : 'local map' }} · {{ m.width }} × {{ m.height }} px · {{ m.ambient }}</span>
        </li>
      </ul>
      <form class="g-card upload" data-testid="map-upload" @submit.prevent="upload()">
        <label class="g-field"><span>Name</span><input v-model="name" maxlength="80" data-testid="map-name" /></label>
        <label class="g-field">
          <span>Kind</span>
          <select v-model="kind" data-testid="map-kind">
            <option value="local">Local map (tactical hexes)</option>
            <option value="world">World map (locations and routes)</option>
          </select>
        </label>
        <label class="g-field">
          <span>Picture (PNG, JPEG or WebP)</span>
          <input type="file" accept="image/png,image/jpeg,image/webp" data-testid="map-file" @change="pick" />
        </label>
        <p v-if="failed" role="alert" class="g-alert">{{ failed }}</p>
        <GButton type="submit" variant="primary" :disabled="!ready">Upload map</GButton>
      </form>
      <section class="g-card upload" aria-labelledby="default-world-title">
        <h2 id="default-world-title">The Default World</h2>
        <p class="hint">A painted world map that comes with Grimoire: land in a sea, six miles to a hex. Set its grid and scale as you like.</p>
        <p v-if="worldFailed" role="alert" class="g-alert" data-testid="default-world-failed">The Default World could not be added.</p>
        <GButton :disabled="busy" data-testid="use-default-world" @click="takeDefaultWorld()">Use the Default World</GButton>
      </section>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.g-list a {
  color: var(--color-gold-high);
}
.hint {
  color: var(--color-text-2);
}
.upload {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
</style>

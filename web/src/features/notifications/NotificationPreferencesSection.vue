<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { ref, watch } from 'vue'
import { getNotificationPreferencesOptions, setNotificationPreferencesMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { NotificationPreference } from '@/infrastructure/api/types.gen'
import NotifyToggle from '@/shared/pwa/NotifyToggle.vue'
import { GButton } from '@/shared/ui'
import { kindLabels } from './kinds'

const prefs = useQuery(getNotificationPreferencesOptions())
const save = useMutation(setNotificationPreferencesMutation())
const rows = ref<NotificationPreference[]>([])
watch(prefs.data, (d) => {
  if (d) rows.value = d.items.map((p) => ({ ...p }))
}, { immediate: true })
</script>

<template>
  <form v-if="rows.length" class="g-card stack" data-testid="notification-preferences" @submit.prevent="save.mutate({ body: { items: rows } })">
    <h2>Notifications</h2>
    <table>
      <thead>
        <tr><th scope="col">Kind</th><th scope="col">In app</th><th scope="col">Devices</th><th scope="col">Email</th></tr>
      </thead>
      <tbody>
        <tr v-for="r in rows" :key="r.kind">
          <th scope="row">{{ kindLabels[r.kind] }}</th>
          <td><input v-model="r.inApp" type="checkbox" :disabled="r.kind === 'security'" :aria-label="`${kindLabels[r.kind]} in app`" :data-testid="`pref-${r.kind}-in-app`" /></td>
          <td><input v-model="r.push" type="checkbox" :aria-label="`${kindLabels[r.kind]} on devices`" :data-testid="`pref-${r.kind}-push`" /></td>
          <td><input v-model="r.email" type="checkbox" :aria-label="`${kindLabels[r.kind]} by email`" :data-testid="`pref-${r.kind}-email`" /></td>
        </tr>
      </tbody>
    </table>
    <p class="hint">Sign-in and security Notifications always show in app. Email other than security comes as a Digest, at most once an hour.</p>
    <NotifyToggle label="Get notifications on this device" done="This device gets the Notifications you chose for devices." />
    <p v-if="save.isSuccess.value" role="status" data-testid="preferences-saved">Saved.</p>
    <p v-if="save.isError.value" role="alert" class="g-alert">The preferences could not be saved.</p>
    <GButton type="submit" :disabled="save.isPending.value">Save the preferences</GButton>
  </form>
</template>

<style scoped>
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
h2 {
  margin: 0;
}
table {
  width: 100%;
  border-collapse: collapse;
}
th,
td {
  padding: 6px 4px;
  border-bottom: 1px solid var(--color-line);
  text-align: center;
}
tbody th,
thead th:first-child {
  text-align: left;
  font-weight: 400;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
  font-size: 14px;
}
</style>

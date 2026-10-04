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
    <p class="hint">Sign-in and security Notifications always show in app. Greyed boxes cannot be changed.</p>
    <aside class="side">
      <section>
        <h3>Email</h3>
        <p class="hint">Email other than security comes as a Digest, at most once an hour.</p>
      </section>
      <section>
        <h3>Devices</h3>
        <NotifyToggle label="Get notifications on this device" done="This device gets the Notifications you chose for devices." />
      </section>
    </aside>
    <p v-if="save.isSuccess.value" role="status" data-testid="preferences-saved">Saved.</p>
    <p v-if="save.isError.value" role="alert" class="g-alert">The preferences could not be saved.</p>
    <GButton type="submit" :disabled="save.isPending.value">Save the preferences</GButton>
  </form>
</template>

<style scoped>
/* The choices are a table; what they mean for email and for this device sits beside it. */
.stack {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 12px 72px;
  align-items: start;
  margin: 0;
  padding: 0;
  border: 0;
}
.stack > * {
  grid-column: 1;
}
.stack > .side {
  display: flex;
  grid-row: 1 / span 6;
  grid-column: 2;
  flex-direction: column;
}
.side section {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 10px;
  padding: 22px 0;
  border-top: 1px solid var(--color-rule);
}
.side section:first-child {
  padding-top: 0;
  border-top: 0;
}
h2,
h3 {
  margin: 0;
}
h3 {
  font-family: var(--font-label);
  font-size: 15px;
  font-weight: 400;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-gold);
}
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 15px;
}
th,
td {
  padding: 14px 12px;
  border-top: 1px solid var(--color-rule);
  border-bottom: 0;
  text-align: center;
}
thead th {
  width: 90px;
  padding-top: 0;
  border-top: 0;
  font-family: var(--font-label);
  font-size: 14px;
  font-weight: 400;
  color: var(--color-text-3);
}
tbody th,
thead th:first-child {
  width: auto;
  padding-left: 0;
  font-weight: 400;
  text-align: left;
}
tbody tr:last-child th,
tbody tr:last-child td {
  border-bottom: 1px solid var(--color-rule);
}
input[type='checkbox'] {
  width: 18px;
  height: 18px;
}
.hint {
  margin: 0;
  font-size: 14px;
  color: var(--color-text-3);
}
.stack > :deep(.g-button) {
  justify-self: start;
}
@media (max-width: 899px) {
  .stack {
    grid-template-columns: minmax(0, 1fr);
  }
  .stack > .side {
    grid-row: auto;
    grid-column: 1;
  }
  th,
  td {
    padding: 12px 4px;
  }
  thead th {
    width: 56px;
  }
}
</style>

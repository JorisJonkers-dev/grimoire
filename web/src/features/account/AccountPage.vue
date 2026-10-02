<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { ref } from 'vue'
import { createAccountInviteMutation, getAccountOptions, setAccountPasswordMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton, GField } from '@/shared/ui'

const account = useQuery(getAccountOptions())
const password = ref('')
const save = useMutation(setAccountPasswordMutation())
const hours = ref(72)
const asAdmin = ref(false)
const invite = useMutation(createAccountInviteMutation())
const inviteLink = (token: string) => `${window.location.origin}/account-invite#${token}`
</script>

<template>
  <main class="g-page">
    <h1>Your Account</h1>
    <p v-if="account.isError.value" role="alert" class="g-alert">Your Account could not be read.</p>
    <template v-else-if="account.data.value">
      <section class="g-card stack" data-testid="account-details">
        <p><strong>{{ account.data.value.nickname }}</strong> · {{ account.data.value.username }} · {{ account.data.value.email }}</p>
        <p v-if="account.data.value.admin">You are an Admin.</p>
      </section>
      <form class="g-card stack" data-testid="password-form" @submit.prevent="save.mutate({ body: { password } })">
        <h2>Password</h2>
        <GField v-model="password" label="New password" type="password" :maxlength="200" autocomplete="new-password" data-testid="account-password" />
        <p v-if="save.isSuccess.value" role="status" data-testid="password-saved">Saved.</p>
        <GButton type="submit" :disabled="password.length < 10 || save.isPending.value">Change the password</GButton>
      </form>
      <form v-if="account.data.value.admin" class="g-card stack" data-testid="invite-form" @submit.prevent="invite.mutate({ body: { hours, admin: asAdmin } })">
        <h2>Invite someone</h2>
        <label class="g-field">
          <span>Open for</span>
          <select v-model.number="hours" data-testid="invite-hours">
            <option :value="24">A day</option>
            <option :value="72">Three days</option>
            <option :value="168">A week</option>
          </select>
        </label>
        <label class="check"><input v-model="asAdmin" type="checkbox" data-testid="invite-admin" /><span>As an Admin</span></label>
        <GButton type="submit" :disabled="invite.isPending.value">Create the invite</GButton>
        <p v-if="invite.data.value" role="status" data-testid="invite-link">Send this link, which works once: <code>{{ inviteLink(invite.data.value.token) }}</code></p>
      </form>
    </template>
  </main>
</template>

<style scoped>
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
h2 {
  margin: 0;
  font-size: 18px;
}
</style>

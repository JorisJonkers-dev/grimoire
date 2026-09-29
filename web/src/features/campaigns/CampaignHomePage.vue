<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  createInviteMutation,
  getCampaignOptions,
  listInvitesOptions,
  removeMemberMutation,
  revokeInviteMutation,
  updateMemberMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Member, Role } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const id = computed(() => String(route.params.id))
const path = computed(() => ({ path: { campaignId: id.value } }))

const campaign = useQuery(computed(() => ({ ...getCampaignOptions(path.value), retry: false })))
const isDM = computed(() => campaign.data.value?.myRole === 'dm')
const invites = useQuery(computed(() => ({ ...listInvitesOptions(path.value), enabled: isDM.value })))

const refresh = () => void client.invalidateQueries()
const failed = ref('')
const onError = (what: string) => () => (failed.value = what)

const newLink = ref('')
const copied = ref(false)
const createInvite = useMutation(createInviteMutation())
const revoke = useMutation(revokeInviteMutation())
const setRole = useMutation(updateMemberMutation())
const remove = useMutation(removeMemberMutation())

function inviteLink() {
  failed.value = ''
  createInvite.mutate(path.value, {
    onSuccess: (inv) => {
      newLink.value = `${window.location.origin}/join#${inv.token}`
      copied.value = false
      refresh()
    },
    onError: onError('The invite link could not be created.'),
  })
}
function revokeInvite(inviteId: string) {
  failed.value = ''
  revoke.mutate({ path: { campaignId: id.value, inviteId } }, { onSuccess: refresh, onError: onError('The invite could not be revoked.') })
}
function changeRole(m: Member, role: Role) {
  failed.value = ''
  setRole.mutate(
    { path: { campaignId: id.value, memberId: m.id }, body: { role } },
    { onSuccess: refresh, onError: onError('That role change was refused. A campaign needs at least one DM.') },
  )
}
function removeMember(m: Member) {
  failed.value = ''
  remove.mutate(
    { path: { campaignId: id.value, memberId: m.id } },
    {
      onSuccess: () => {
        if (m.isMe) void router.push({ name: 'campaigns' })
        else refresh()
      },
      onError: onError('That member could not be removed. A campaign needs at least one DM.'),
    },
  )
}
async function copy() {
  await navigator.clipboard.writeText(newLink.value)
  copied.value = true
}
const expires = (iso: string) => new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short' })
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'campaigns' }" class="back">← Campaigns</RouterLink>
    <p v-if="campaign.isPending.value">Opening the campaign…</p>
    <p v-else-if="campaign.isError.value" role="alert" class="g-alert" data-testid="campaign-missing">
      This campaign does not exist, or you are not one of its members.
    </p>
    <template v-else-if="campaign.data.value">
      <header>
        <h1>{{ campaign.data.value.name }}</h1>
        <p class="sub">
          {{ campaign.data.value.ruleset === 'srd-2024' ? '2024 rules' : '2014 rules' }} · you are
          {{ isDM ? 'a DM' : 'a Player' }}
        </p>
      </header>
      <p v-if="failed" role="alert" class="g-alert" data-testid="campaign-error">{{ failed }}</p>

      <section class="g-card">
        <h2>Members</h2>
        <ul class="g-list" data-testid="member-list">
          <li v-for="m in campaign.data.value.members" :key="m.id" class="member">
            <span class="who">
              {{ m.displayName }}<template v-if="m.isMe"> (you)</template>
            </span>
            <span class="g-tag">{{ m.role === 'dm' ? 'DM' : 'Player' }}</span>
            <span v-if="isDM" class="actions">
              <GButton v-if="m.role === 'player'" @click="changeRole(m, 'dm')">Make co-DM</GButton>
              <GButton v-else @click="changeRole(m, 'player')">Make Player</GButton>
              <GButton v-if="!m.isMe" variant="danger" @click="removeMember(m)">Remove</GButton>
            </span>
          </li>
        </ul>
        <GButton v-if="!isDM" variant="danger" data-testid="leave" @click="removeMember(campaign.data.value.me)">Leave campaign</GButton>
      </section>

      <section v-if="isDM" class="g-card" data-testid="invites">
        <h2>Invite players</h2>
        <p>Anyone with the link joins as a Player. Links last a week.</p>
        <GButton variant="primary" data-testid="create-invite" @click="inviteLink()">Create invite link</GButton>
        <div v-if="newLink" class="link">
          <label class="g-field">
            <span>New link, shown once</span>
            <input :value="newLink" readonly data-testid="invite-link" />
          </label>
          <GButton @click="copy()">{{ copied ? 'Copied' : 'Copy' }}</GButton>
        </div>
        <ul class="g-list" data-testid="invite-list">
          <li v-for="inv in invites.data.value ?? []" :key="inv.id" class="member">
            <span class="who">By {{ inv.createdBy }}, until {{ expires(inv.expiresAt) }}</span>
            <GButton variant="danger" @click="revokeInvite(inv.id)">Revoke</GButton>
          </li>
        </ul>
      </section>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.sub {
  margin: 4px 0 0;
  font-family: var(--font-flavour);
  font-style: italic;
  color: var(--color-text-2);
}
h2 {
  margin: 0 0 10px;
  font-family: var(--font-display);
  font-size: 18px;
}
section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.member {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  border-bottom: 1px solid var(--color-line);
}
.who {
  flex: 1 1 140px;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.link {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.link .g-field {
  flex: 1 1 240px;
  min-width: 0;
}
</style>

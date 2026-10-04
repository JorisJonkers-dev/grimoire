<script setup lang="ts">
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import {
  beginTwoStepMutation,
  confirmTwoStepMutation,
  disableTwoStepMutation,
  getAccountQueryKey,
  resetRecoveryCodesMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Account } from '@/infrastructure/api/types.gen'
import { GButton, GCodeField, GQrCode } from '@/shared/ui'

const props = defineProps<{ account: Account }>()
const client = useQueryClient()
const refresh = () => client.invalidateQueries({ queryKey: getAccountQueryKey() })
const begin = useMutation(beginTwoStepMutation())
const confirm = useMutation(confirmTwoStepMutation())
const disable = useMutation(disableTwoStepMutation())
const reset = useMutation(resetRecoveryCodesMutation())
const code = ref('')
const codes = ref<string[]>([])
const setup = computed(() => (props.account.twoStep ? undefined : begin.data.value))
const wrong = computed(() => confirm.isError.value || disable.isError.value || reset.isError.value)

function start() {
  codes.value = []
  begin.mutate({})
}
function turnOn() {
  confirm.mutate({ body: { code: code.value } }, { onSuccess: (out) => {
      codes.value = out.codes
      code.value = ''
      void refresh()
    } })
}
function turnOff() {
  disable.mutate({ body: { code: code.value } }, { onSuccess: () => {
      code.value = ''
      begin.reset()
      void refresh()
    } })
}
function newCodes() {
  reset.mutate({ body: { code: code.value } }, { onSuccess: (out) => {
      codes.value = out.codes
      code.value = ''
      void refresh()
    } })
}
</script>

<template>
  <section class="stack" aria-label="Two-step sign-in" data-testid="two-step-section">
    <p v-if="account.admin && !account.adminPowers" role="alert" class="g-alert" data-testid="admin-needs-two-step">
      Turn on two-step sign-in to use your Admin powers.
    </p>
    <div v-if="codes.length" class="codes" data-testid="recovery-codes">
      <p><strong>Save these recovery codes now.</strong> Each signs you in once if you lose your phone; they are not shown again.</p>
      <ol>
        <li v-for="c in codes" :key="c"><code>{{ c }}</code></li>
      </ol>
    </div>
    <template v-if="account.twoStep">
      <p data-testid="two-step-on">On. {{ account.recoveryCodesLeft }} recovery codes left.</p>
      <form class="stack" @submit.prevent>
        <GCodeField v-model="code" label="A current code, to make a change" data-testid="two-step-manage-code" />
        <p v-if="wrong" role="alert" class="g-alert" data-testid="two-step-change-failed">That code is wrong or was already used.</p>
        <div class="row">
          <GButton type="button" :disabled="code.length !== 6 || reset.isPending.value" data-testid="two-step-reset" @click="newCodes">New recovery codes</GButton>
          <GButton type="button" variant="danger" :disabled="code.length !== 6 || disable.isPending.value" data-testid="two-step-disable" @click="turnOff">Turn off</GButton>
        </div>
      </form>
    </template>
    <template v-else-if="setup">
      <p>Scan this with an authenticator app, or open it on this phone, then enter the code it shows.</p>
      <a :href="setup.uri" class="qr-link" data-testid="two-step-uri">
        <GQrCode :value="setup.uri" label="Two-step sign-in QR code" />
        <span>Open in an authenticator app</span>
      </a>
      <p class="secret">Or type this key: <code data-testid="two-step-secret">{{ setup.secret }}</code></p>
      <form class="stack" data-testid="two-step-confirm" @submit.prevent="turnOn">
        <GCodeField v-model="code" data-testid="two-step-confirm-code" />
        <p v-if="confirm.isError.value" role="alert" class="g-alert">That code does not match. Check the time on your phone and try the next one.</p>
        <GButton type="submit" variant="primary" :disabled="code.length !== 6 || confirm.isPending.value">Turn on two-step</GButton>
      </form>
    </template>
    <template v-else>
      <p>Off. With two-step on, signing in with your password also asks for a code from an authenticator app.</p>
      <p v-if="begin.isError.value" role="alert" class="g-alert">Two-step could not be started. Try again shortly.</p>
      <GButton type="button" :disabled="begin.isPending.value" data-testid="two-step-begin" @click="start">Set up two-step</GButton>
    </template>
  </section>
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
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.qr-link {
  display: flex;
  flex-direction: column;
  align-self: flex-start;
  gap: 4px;
  color: var(--color-gold);
}
.secret code {
  overflow-wrap: anywhere;
}
.codes ol {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
  gap: 4px 16px;
  margin: 0;
  padding-left: 24px;
}
</style>

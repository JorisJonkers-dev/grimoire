<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { getRollOptions, getRollQueryKey } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { RollRequest } from '@/infrastructure/api/types.gen'
import RollCard from '@/features/rolls/RollCard.vue'

const props = defineProps<{ campaignId: string; rollId: string }>()
const client = useQueryClient()
const options = { path: { campaignId: props.campaignId, rollId: props.rollId } }
const roll = useQuery({ ...getRollOptions(options), retry: false })
function updated(r: RollRequest) {
  client.setQueryData(getRollQueryKey(options), r)
}
</script>

<template>
  <RollCard v-if="roll.data.value" :roll="roll.data.value" :campaign-id="campaignId" @updated="updated" />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { type RouteLocationRaw, useRouter } from 'vue-router'
import { GButton } from '@/shared/ui'

// Copies the address of a page of the compendium, so it can be shared.
const props = defineProps<{ to: RouteLocationRaw }>()
const router = useRouter()
const address = computed(() => new URL(router.resolve(props.to).href, window.location.origin).href)
const copied = ref(false)
// byHand shows the address where the browser will not copy it.
const byHand = ref(false)
watch(address, () => { copied.value = byHand.value = false })
async function copy() {
  try {
    await navigator.clipboard.writeText(address.value)
    copied.value = true
  } catch {
    byHand.value = true
  }
}
</script>

<template>
  <div class="copy">
    <GButton data-testid="copy-link" @click="copy">Copy link</GButton>
    <span v-if="copied" role="status" class="done" data-testid="link-copied">Link copied.</span>
    <label v-if="byHand" class="g-field">
      <span>Copy this address</span>
      <input :value="address" readonly data-testid="link-to-copy" @focus="($event.target as HTMLInputElement).select()" />
    </label>
  </div>
</template>

<style scoped>
.copy {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}
.done {
  font-size: 14px;
  color: var(--color-success);
}
</style>

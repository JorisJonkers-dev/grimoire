<script setup lang="ts">
defineProps<{ percent: number; damage: string; detail: string }>()
</script>

<template>
  <div class="hit" role="status">
    <span class="percent">{{ Math.round(percent) }}%</span>
    <span class="text">
      <span class="damage">to hit · {{ damage }}</span>
      <span class="detail">{{ detail }}</span>
    </span>
  </div>
</template>

<style scoped>
.hit {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 16px;
  border: 1px solid var(--color-gold);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
}
.percent {
  font-family: var(--font-display);
  font-weight: 700;
  font-size: 30px;
  color: var(--color-gold-high);
}
.text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.damage {
  font-weight: 700;
}
.detail {
  font-size: 14px;
  color: var(--color-text-2);
}
</style>

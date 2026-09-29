<script setup lang="ts">
defineProps<{ action: boolean; bonus: boolean; reaction: boolean; movementLeft: number; movementTotal: number }>()
</script>

<template>
  <div class="pips" role="group" aria-label="Action economy">
    <span class="pip" :data-available="action">
      <svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
        <circle cx="8" cy="8" r="6.5" :fill="action ? 'var(--color-gold-high)' : 'none'" stroke="var(--color-gold-high)" stroke-width="1.5" />
      </svg>
      Action<span class="sr">{{ action ? ' available' : ' used' }}</span>
    </span>
    <span class="pip" :data-available="bonus">
      <svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
        <polygon points="8,1.5 14.5,8 8,14.5 1.5,8" :fill="bonus ? 'var(--color-gold-high)' : 'none'" stroke="var(--color-gold-high)" stroke-width="1.5" />
      </svg>
      Bonus<span class="sr">{{ bonus ? ' available' : ' used' }}</span>
    </span>
    <span class="pip" :data-available="reaction">
      <svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
        <path d="M8 1.5 L14 5 L14 11 L8 14.5 L2 11 L2 5 Z" :fill="reaction ? 'var(--color-gold-high)' : 'none'" stroke="var(--color-gold-high)" stroke-width="1.5" />
      </svg>
      Reaction<span class="sr">{{ reaction ? ' available' : ' used' }}</span>
    </span>
    <span class="movement">{{ movementLeft }} / {{ movementTotal }} ft</span>
  </div>
</template>

<style scoped>
.pips {
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 15px;
}
.pip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.pip[data-available='false'] {
  color: var(--color-text-3);
}
.movement {
  margin-left: auto;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
}
.sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
}
</style>

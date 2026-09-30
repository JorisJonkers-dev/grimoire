<script setup lang="ts">
defineProps<{ focus: { x: number; y: number }; zoom: number; ping: { x: number; y: number; n: number } | null }>()
</script>

<template>
  <div class="camera" data-testid="camera">
    <div
      class="world"
      :style="{ transform: `translate(${String(-focus.x * zoom)}px, ${String(-focus.y * zoom)}px) scale(${String(zoom)})` }"
      data-testid="camera-world"
    >
      <slot />
      <span
        v-if="ping"
        :key="ping.n"
        class="ping"
        :style="{ left: `${String(ping.x)}px`, top: `${String(ping.y)}px` }"
        role="status"
        aria-label="The DM pinged here"
        data-testid="ping"
      ></span>
    </div>
  </div>
</template>

<style scoped>
.camera {
  position: relative;
  width: 100%;
  height: calc(100vh - 140px);
  min-height: 320px;
  overflow: hidden;
}
.world {
  position: absolute;
  top: 50%;
  left: 50%;
  transform-origin: 0 0;
  transition: transform 400ms ease;
}
.world :deep(.board-scroll),
.world :deep(.hex-scroll) {
  overflow: visible;
  max-width: none;
}
.ping {
  position: absolute;
  width: 60px;
  height: 60px;
  margin: -30px 0 0 -30px;
  border: 4px solid var(--color-gold-high);
  border-radius: 50%;
  animation: ping 1.5s ease-out 2;
  pointer-events: none;
}
@keyframes ping {
  from {
    transform: scale(0.3);
    opacity: 1;
  }
  to {
    transform: scale(2);
    opacity: 0;
  }
}
@media (prefers-reduced-motion: reduce) {
  .world {
    transition: none;
  }
  .ping {
    animation: none;
  }
}
</style>

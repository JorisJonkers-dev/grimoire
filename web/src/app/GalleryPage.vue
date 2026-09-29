<script setup lang="ts">
import { ConditionChip, DieFace, EconomyPips, GButton, HitChance, HotbarSlot, ReactionTimer, TokenBadge } from '@/shared/ui'
</script>

<template>
  <main class="gallery">
    <h1>Component gallery</h1>
    <section aria-labelledby="g-buttons">
      <h2 id="g-buttons">Buttons</h2>
      <div class="row">
        <GButton variant="primary">End turn</GButton>
        <GButton>Undo move</GButton>
        <GButton variant="danger">Blackout TV</GButton>
        <GButton disabled>Unavailable</GButton>
      </div>
    </section>
    <section aria-labelledby="g-hotbar">
      <h2 id="g-hotbar">Hotbar</h2>
      <div class="row">
        <HotbarSlot label="Longbow" />
        <HotbarSlot label="Hunter's Mark" kind="bonus" />
        <HotbarSlot label="Shortsword" suggested />
        <HotbarSlot label="Disengage" :available="false" reason="No action left" />
      </div>
      <EconomyPips :action="true" :bonus="true" :reaction="false" :movement-left="10" :movement-total="30" />
    </section>
    <section aria-labelledby="g-tokens">
      <h2 id="g-tokens">Tokens</h2>
      <div class="row">
        <TokenBadge name="Aria Vale" allegiance="party" active />
        <TokenBadge name="Skeleton" allegiance="enemy" />
        <TokenBadge name="Wight" allegiance="enemy" hidden />
        <TokenBadge name="Brom" allegiance="party" :size="84" />
      </div>
    </section>
    <section aria-labelledby="g-state">
      <h2 id="g-state">Conditions and odds</h2>
      <div class="row">
        <ConditionChip label="Blessed" detail="7 rounds" />
        <ConditionChip label="Prone" tone="bane" />
      </div>
      <HitChance :percent="92" damage="1d8+4 piercing" detail="5–12 · advantage, Bless, half cover" />
      <ReactionTimer :seconds-left="7" :total="10" />
    </section>
    <section aria-labelledby="g-dice">
      <h2 id="g-dice">Dice</h2>
      <div class="row tray">
        <DieFace :sides="20" :value="14" state="kept" />
        <DieFace :sides="20" :value="7" state="dropped" />
        <DieFace :sides="4" />
        <DieFace :sides="6" :value="5" />
        <DieFace :sides="8" :value="3" />
      </div>
    </section>
  </main>
</template>

<style scoped>
.gallery {
  display: flex;
  flex-direction: column;
  gap: 24px;
  max-width: 960px;
  margin: 0 auto;
  padding: 24px 16px;
}
h1 {
  margin: 0;
  font-family: var(--font-display);
}
h2 {
  margin: 0 0 12px;
  font-family: var(--font-display);
  font-size: 15px;
  letter-spacing: 0.14em;
  color: var(--color-gold);
  text-transform: uppercase;
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  margin-bottom: 12px;
}
.tray {
  padding: 12px;
  border-radius: var(--radius-lg);
  background: var(--color-felt);
}
</style>

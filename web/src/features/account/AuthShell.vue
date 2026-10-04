<script setup lang="ts">
// The frame around signing in: the brand on one side, one centred column for the form on the other.
</script>

<template>
  <div class="auth">
    <aside class="brand" data-testid="auth-brand">
      <svg class="sigil" viewBox="0 0 520 520" aria-hidden="true">
        <circle cx="260" cy="260" r="200" />
        <circle cx="260" cy="260" r="160" class="faint" />
        <polygon points="260,70 425,355 95,355" />
        <polygon points="260,450 95,165 425,165" />
        <circle cx="260" cy="260" r="34" class="bright" />
      </svg>
      <RouterLink :to="{ name: 'home' }" class="name">Grimoire</RouterLink>
      <p class="tagline">The table is set. Take your seat.</p>
    </aside>
    <main class="panel">
      <div class="column" data-testid="auth-column">
        <slot />
      </div>
    </main>
  </div>
</template>

<style scoped>
.auth {
  min-height: 100vh;
  min-height: 100dvh;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  background: var(--color-ground);
  color: var(--color-text);
}
.brand {
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  gap: 12px;
  padding: 64px;
  border-right: 1px solid var(--color-line);
  background: radial-gradient(ellipse at 30% 30%, var(--color-ink) 0%, var(--color-surface) 55%, var(--color-ground) 100%);
}
.sigil {
  position: absolute;
  top: 60px;
  left: 80px;
  width: min(520px, 80%);
  opacity: 0.5;
  fill: none;
  stroke: var(--color-bronze);
  stroke-width: 1.4;
}
.sigil .faint {
  stroke: var(--color-line);
}
.sigil .bright {
  stroke: var(--color-gold);
}
.name {
  position: relative;
  font-family: var(--font-display);
  font-weight: 700;
  font-size: 56px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  text-decoration: none;
  color: var(--color-gold);
}
.tagline {
  position: relative;
  margin: 0;
  font-family: var(--font-flavour);
  font-style: italic;
  font-size: 22px;
  color: var(--color-parchment);
}
.panel {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px;
}
.column {
  width: 100%;
  max-width: 420px;
  display: flex;
  flex-direction: column;
  gap: 22px;
}
/* On a phone the brand is a band across the top and the form fills the screen under it. */
@media (max-width: 899px) {
  .auth {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: auto 1fr;
  }
  .brand {
    min-height: 200px;
    gap: 4px;
    padding: 0 22px 18px;
    border-right: 0;
    background: radial-gradient(ellipse at 40% 30%, var(--color-ink) 0%, var(--color-surface) 60%, var(--color-ground) 100%);
  }
  .sigil {
    top: -20px;
    right: -30px;
    left: auto;
    width: 240px;
    stroke-width: 2.5;
  }
  .name {
    font-size: 34px;
  }
  .tagline {
    font-size: 17px;
  }
  .panel {
    align-items: flex-start;
    padding: 22px 22px 28px;
  }
  .column {
    max-width: 480px;
    gap: 18px;
  }
}
</style>

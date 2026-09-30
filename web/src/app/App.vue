<template>
  <div class="shell">
    <header class="bar">
      <RouterLink :to="{ name: 'home' }" class="brand">Grimoire</RouterLink>
      <nav aria-label="Main">
        <RouterLink :to="{ name: 'campaigns' }">Campaigns</RouterLink>
        <RouterLink :to="{ name: 'spells' }">Compendium</RouterLink>
      </nav>
    </header>
    <RouterView />
    <footer class="credit">
      Grimoire by <a href="https://jorisjonkers.dev">Joris Jonkers</a> ·
      <RouterLink :to="{ name: 'attribution' }">SRD content under CC-BY-4.0</RouterLink> ·
      <RouterLink :to="{ name: 'automation' }">Automation coverage</RouterLink>
    </footer>
  </div>
</template>

<style scoped>
.shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
.bar {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 0 16px;
  min-height: 56px;
  border-bottom: 1px solid var(--color-line);
  background: var(--color-surface);
}
.brand {
  font-family: var(--font-display);
  font-weight: 700;
  letter-spacing: 0.2em;
  color: var(--color-gold);
  text-decoration: none;
}
nav {
  display: flex;
  gap: 16px;
}
nav a {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  color: var(--color-text);
}
.credit {
  margin-top: auto;
  padding: 16px;
  font-size: 14px;
  color: var(--color-text-2);
  border-top: 1px solid var(--color-line);
}
.credit a {
  color: var(--color-gold-high);
}
</style>

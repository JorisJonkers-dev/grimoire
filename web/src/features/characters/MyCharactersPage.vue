<script setup lang="ts">
import { GAvatar } from '@/shared/ui'
import { useQuery } from '@tanstack/vue-query'
import { listMyCharactersOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'

const mine = useQuery(listMyCharactersOptions())
</script>

<template>
  <main class="g-page">
    <header class="g-headline">
      <span class="g-eyebrow">Your heroes</span>
      <h1>Your Characters</h1>
    </header>
    <p class="g-lede">A Character can play in several Campaigns. Name, Portrait and Backstory are shared; level, hit points and gear are kept per Campaign.</p>
    <p v-if="mine.isError.value" role="alert" class="g-alert">Your Characters could not be read.</p>
    <table v-else-if="mine.data.value?.items.length" data-testid="my-characters">
      <thead>
        <tr>
          <th scope="col">Character</th>
          <th scope="col">Plays in</th>
          <th scope="col" class="open"><span class="sr">Open</span></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="c in mine.data.value.items" :key="c.id">
          <td>
            <span class="hero">
              <GAvatar :name="c.name" :size="48" />
              <span class="who">
                <RouterLink :to="{ name: 'my-character', params: { characterId: c.id } }" class="name">{{ c.name }}</RouterLink>
                <span class="dim">{{ c.species }} {{ c.class }} · {{ c.background }}</span>
              </span>
            </span>
          </td>
          <td>
            <template v-if="c.campaigns.length">
              <span v-for="e in c.campaigns" :key="e.campaignName" class="plays">{{ e.campaignName }} <span class="dim">· level {{ e.level }}</span></span>
            </template>
            <span v-else class="dim">Not in a Campaign yet</span>
          </td>
          <td class="open">
            <RouterLink :to="{ name: 'my-character', params: { characterId: c.id } }" class="chevron" aria-hidden="true" tabindex="-1">
              <svg width="12" height="22" viewBox="0 0 12 22"><path d="M2 2 L10 11 L2 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" /></svg>
            </RouterLink>
          </td>
        </tr>
      </tbody>
    </table>
    <p v-else-if="mine.isSuccess.value" data-testid="my-characters-none">
      No Characters yet. Make one from a Campaign's party page.
    </p>
  </main>
</template>

<style scoped>
.hero {
  display: flex;
  align-items: center;
  gap: 14px;
}
.who {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.name {
  font-family: var(--font-display);
  font-weight: 600;
  font-size: 18px;
  color: var(--color-text);
  text-decoration: none;
}
.name:hover {
  color: var(--color-gold-high);
}
.plays {
  display: block;
}
.dim {
  color: var(--color-text-3);
  font-size: 13px;
}
.plays .dim {
  font-size: 15px;
}
.open {
  width: 64px;
  text-align: right;
}
.chevron {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  color: var(--color-text-3);
}
.sr {
  position: absolute;
  left: -9999px;
}
/* On a phone each Character is a block: who it is, then where it plays. */
@media (max-width: 899px) {
  thead {
    display: none;
  }
  tr {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    padding: 12px 0;
    border-top: 1px solid var(--color-rule);
  }
  td {
    padding: 0;
    border: 0;
  }
  td:nth-child(2) {
    grid-column: 1;
    grid-row: 2;
    padding: 6px 0 0 62px;
    overflow-wrap: anywhere;
  }
  td.open {
    grid-row: 1 / span 2;
    align-self: center;
    width: auto;
  }
}
</style>

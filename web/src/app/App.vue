<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import NotificationBell from '@/features/notifications/NotificationBell.vue'
import { getAccountOptions, signOutMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { restoreAccessibility } from '@/shared/a11y/settings'
import KeyMap from '@/shared/input/KeyMap.vue'
import { useInput } from '@/shared/input/useInput'
import { useOnline } from '@/shared/pwa/online'

// How this device is set to look is taken up before anything is drawn.
restoreAccessibility()
// Shortcut keys and a gamepad work on every page, the ones with no header too.
const keys = useInput()
const route = useRoute()
const router = useRouter()
const online = useOnline()
const bare = computed(() => route.meta.bare === true)
// A live page belongs to one Session: going to another Session, as a split party does, makes it anew.
const pageKey = computed(() => String(route.params.sid ?? ''))
const account = useQuery({ ...getAccountOptions(), retry: false })
const client = useQueryClient()
const signOut = useMutation(signOutMutation())
// The header's search box hands what was typed to the search page.
const sought = ref('')
function seek() {
  const q = sought.value.trim()
  sought.value = ''
  void router.push({ name: 'search', query: q ? { q } : {} })
}
function leave() {
  menu.value = false
  signOut.mutate({}, { onSuccess: () => { client.clear(); void router.push({ name: 'sign-in' }) } })
}
// The account's own places open from its picture.
const menu = ref(false)
const nickname = computed(() => account.data.value?.nickname ?? '')
// The five places of the app, the same in the bar and in a phone's tab bar; the one I am in is marked.
const places = computed(() => {
  const signedIn = Boolean(account.data.value)
  const at = route.path
  const all = [
    { label: 'Dashboard', to: { name: 'home' }, on: at === '/', show: true, testid: 'dashboard-link' },
    { label: 'Campaigns', to: { name: 'campaigns' }, on: at.startsWith('/campaigns') || at.startsWith('/join'), show: true, testid: 'campaigns-link' },
    { label: 'Characters', to: { name: 'my-characters' }, on: at.startsWith('/characters'), show: signedIn, testid: 'characters-link' },
    { label: 'Library', to: { name: 'library' }, on: at.startsWith('/library') || at.startsWith('/shared-library'), show: signedIn, testid: 'library-link' },
    { label: 'Compendium', to: { name: 'spells' }, on: at.startsWith('/compendium'), show: true, testid: 'compendium-link' },
  ]
  return all.filter((p) => p.show)
})
router.afterEach(() => { menu.value = false })
</script>

<template>
  <KeyMap v-if="keys.open.value" :here="keys.rows.value" @close="keys.show(false)" />
  <!-- While the list of keys shows, the page behind it takes no focus and no keys. -->
  <div class="page" :inert="keys.open.value || undefined">
    <RouterView v-if="bare" :key="pageKey" />
    <div v-else :class="['shell', { 'shell--live': route.name === 'session' }]">
      <header class="bar">
        <RouterLink :to="{ name: 'home' }" class="brand">Grimoire</RouterLink>
        <nav aria-label="Main" class="places">
          <RouterLink v-for="p in places" :key="p.label" :to="p.to" :class="{ on: p.on }" :aria-current="p.on ? 'page' : undefined" :data-testid="p.testid">
            <span class="mark" aria-hidden="true" />{{ p.label }}
          </RouterLink>
        </nav>
        <div class="tools">
          <form v-if="route.name !== 'search'" role="search" aria-label="Search everything" class="seek" data-testid="header-search" @submit.prevent="seek">
            <svg width="15" height="15" viewBox="0 0 16 16" aria-hidden="true"><circle cx="7" cy="7" r="5" fill="none" stroke="currentColor" stroke-width="1.6" /><path d="M11 11 L14.5 14.5" stroke="currentColor" stroke-width="1.6" /></svg>
            <input v-model="sought" type="search" maxlength="80" placeholder="Search everything" aria-label="Search everything" aria-keyshortcuts="/" data-testid="header-search-input" />
            <kbd aria-hidden="true">/</kbd>
          </form>
          <RouterLink v-if="route.name !== 'search'" :to="{ name: 'search' }" class="icon seek-link" aria-label="Search everything" aria-keyshortcuts="/" data-testid="search-link">
            <svg width="19" height="19" viewBox="0 0 16 16" aria-hidden="true"><circle cx="7" cy="7" r="5" fill="none" stroke="currentColor" stroke-width="1.6" /><path d="M11 11 L14.5 14.5" stroke="currentColor" stroke-width="1.6" /></svg>
          </RouterLink>
          <template v-if="account.data.value">
            <RouterLink :to="{ name: 'conversations' }" class="icon talk" aria-label="Talk" data-testid="conversations-link">
              <svg width="21" height="21" viewBox="0 0 20 20" aria-hidden="true"><path d="M3 4 H17 V13 H9 L5 16 V13 H3 Z" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round" /></svg>
            </RouterLink>
            <NotificationBell />
            <div class="me" data-testid="account-menu">
              <button type="button" class="avatar" :aria-expanded="menu" aria-controls="account-places" :aria-label="`Account menu, ${nickname}`" data-testid="account-menu-toggle" @click="menu = !menu">
                <span aria-hidden="true">{{ nickname.slice(0, 1).toUpperCase() }}</span>
              </button>
              <ul v-show="menu" id="account-places" class="menu">
                <li><RouterLink :to="{ name: 'account' }" class="who" data-testid="account-link">{{ nickname }}</RouterLink></li>
                <li><RouterLink :to="{ name: 'friends' }" data-testid="friends-link">Friends</RouterLink></li>
                <li><RouterLink :to="{ name: 'dice-sets' }" data-testid="dice-sets-link">Dice</RouterLink></li>
                <li><RouterLink :to="{ name: 'accessibility' }">Accessibility</RouterLink></li>
                <li v-if="account.data.value.adminPowers"><RouterLink :to="{ name: 'admin' }" data-testid="admin-link">Admin</RouterLink></li>
                <li><button type="button" data-testid="sign-out" @click="leave">Sign out</button></li>
              </ul>
            </div>
          </template>
          <RouterLink v-else-if="route.name !== 'sign-in'" :to="{ name: 'sign-in' }" class="enter" data-testid="sign-in-link">Sign in</RouterLink>
        </div>
      </header>
      <p v-if="!online" role="status" class="offline" data-testid="offline">
        You are offline. The compendium and your Character sheets still open from this device; live play picks up again when the connection returns.
      </p>
      <RouterView :key="pageKey" />
      <footer class="credit">
        <span>Grimoire by <a href="https://jorisjonkers.dev">Joris Jonkers</a> · rules text from the SRD 5.2, CC BY 4.0</span>
        <span class="links">
          <RouterLink :to="{ name: 'attribution' }">SRD content under CC-BY-4.0</RouterLink>
          <RouterLink :to="{ name: 'automation' }">Automation coverage</RouterLink>
          <RouterLink :to="{ name: 'accessibility' }" data-testid="accessibility-link">Accessibility</RouterLink>
        </span>
      </footer>
      <!-- On a phone the five places are a bar along the bottom edge. -->
      <nav aria-label="Places" class="tabs" data-testid="tab-bar">
        <RouterLink v-for="p in places" :key="p.label" :to="p.to" :class="{ on: p.on }" :aria-current="p.on ? 'page' : undefined">
          <span class="mark" aria-hidden="true" />{{ p.label }}
        </RouterLink>
      </nav>
    </div>
  </div>
</template>

<style scoped>
/* A wrapper for making the page inert; it lays nothing out itself. */
.page {
  display: contents;
}
.shell {
  min-height: 100vh;
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
}
/* The bar: dark leather with a gold rule set a little above its lower edge. */
.bar {
  display: flex;
  align-items: center;
  gap: 36px;
  height: var(--size-bar);
  flex: none;
  box-sizing: border-box;
  padding: 0 32px;
  background: var(--color-bar);
  box-shadow:
    inset 0 -1px 0 var(--color-bar-shade),
    inset 0 -4px 0 var(--color-bar),
    inset 0 -5px 0 var(--color-brass-line);
  color: var(--color-bar-text);
}
.brand {
  display: flex;
  align-items: center;
  height: 40px;
  font-family: var(--font-display);
  font-weight: 700;
  font-size: 24px;
  line-height: 1;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--color-bar-brand);
}
.places {
  display: flex;
  align-items: center;
  gap: 4px;
  height: 40px;
  /* At a large text size on a narrow screen the places scroll inside the bar rather than widen the page. */
  min-width: 0;
  overflow-x: auto;
  scrollbar-width: none;
}
.places a {
  flex: none;
  display: flex;
  align-items: center;
  gap: 9px;
  height: 40px;
  padding: 0 12px;
  font-family: var(--font-label);
  font-size: 17px;
  line-height: 1;
  color: var(--color-bar-text);
}
.places a.on,
.tabs a.on {
  color: var(--color-bar-brand);
}
/* A small diamond marks the place I am in. */
.mark {
  width: 7px;
  height: 7px;
  flex: none;
  box-sizing: border-box;
  transform: rotate(45deg);
}
.on .mark {
  background: var(--color-bar-brand);
}
.tools {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 6px;
}
.seek {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 260px;
  height: 38px;
  box-sizing: border-box;
  margin-right: 10px;
  padding: 0 12px;
  border: 1px solid var(--color-bar-field-edge);
  border-radius: var(--radius-control);
  background: var(--color-bar-field);
  color: var(--color-bar-field-text);
}
.seek:focus-within {
  border-color: var(--color-brass-line);
}
.seek input {
  flex: 1;
  min-width: 0;
  border: 0;
  background: transparent;
  color: var(--color-text);
  font-family: var(--font-ui);
  font-size: 15px;
}
.seek input:focus {
  outline: none;
}
.seek input::placeholder {
  color: var(--color-bar-field-text);
}
.seek kbd {
  font-family: var(--font-ui);
  font-size: 12px;
  color: var(--color-text-3);
}
.icon {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-bar-text);
}
.icon:hover {
  color: var(--color-bar-brand);
}
.seek-link {
  display: none;
}
.me {
  position: relative;
  margin-left: 4px;
}
.avatar {
  width: 40px;
  height: 40px;
  padding: 0;
  border: 0;
  background: transparent;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}
.avatar span {
  width: 34px;
  height: 34px;
  box-sizing: border-box;
  border-radius: 50%;
  border: 1px solid var(--color-brass-line);
  background: var(--color-bar-avatar);
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: var(--font-display);
  font-weight: 700;
  font-size: 15px;
  color: var(--color-bar-brand);
}
.menu {
  position: absolute;
  z-index: 30;
  top: 48px;
  right: 0;
  min-width: 200px;
  margin: 0;
  padding: 6px 0;
  list-style: none;
  border: 1px solid var(--color-edge);
  border-radius: var(--radius-panel);
  background: var(--color-surface);
  box-shadow: 0 14px 32px rgb(0 0 0 / 55%);
}
.menu a,
.menu button {
  display: flex;
  align-items: center;
  width: 100%;
  min-height: 40px;
  box-sizing: border-box;
  padding: 0 16px;
  border: 0;
  background: none;
  color: var(--color-text);
  font-family: var(--font-ui);
  font-size: 15px;
  text-align: left;
  cursor: pointer;
}
.menu a:hover,
.menu button:hover {
  background: var(--color-selected);
  color: var(--color-gold-high);
}
.menu .who {
  font-family: var(--font-display);
  font-weight: 600;
  border-bottom: 1px solid var(--color-rule);
}
.enter {
  padding: 0 8px;
  font-family: var(--font-label);
  font-size: 16px;
  color: var(--color-bar-brand);
}
.offline {
  margin: 0;
  padding: 8px var(--gutter);
  background: var(--color-surface);
  border-bottom: 1px solid var(--color-gold);
  color: var(--color-gold-high);
}
/* The footer spans the whole window, whatever the page above it does. */
.credit {
  margin-top: auto;
  width: 100%;
  box-sizing: border-box;
  padding: 16px var(--gutter);
  border-top: 1px solid var(--color-rule);
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px 20px;
  font-size: 13px;
  color: var(--color-text-3);
}
.credit a {
  color: var(--color-text-2);
}
.credit .links a,
.brand,
.icon,
.enter,
.menu a {
  text-decoration: none;
}
.credit a:hover {
  color: var(--color-gold-high);
}
.credit .links {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 20px;
}
.tabs {
  display: none;
}
/* Where the bar gets tight the search box folds into its glass. */
@media (max-width: 1239px) {
  .bar {
    gap: 20px;
  }
  .seek {
    display: none;
  }
  .seek-link {
    display: flex;
  }
}
/* A phone: a shorter bar with the brand and three round controls, and the places along the bottom. */
@media (max-width: 899px) {
  .bar {
    height: 60px;
    gap: 8px;
    padding: 0 12px 0 16px;
    box-shadow:
      inset 0 -3px 0 var(--color-bar),
      inset 0 -4px 0 var(--color-brass-line);
  }
  .brand {
    flex: 1;
    font-size: 19px;
    letter-spacing: 0.14em;
  }
  .places,
  .talk {
    display: none;
  }
  .tools {
    gap: 0;
  }
  .icon,
  .avatar {
    width: 42px;
    height: 42px;
  }
  .me {
    margin-left: 0;
  }
  .shell {
    padding-bottom: 68px;
  }
  .tabs {
    position: fixed;
    z-index: 25;
    left: 0;
    right: 0;
    bottom: 0;
    height: 68px;
    display: grid;
    grid-auto-flow: column;
    grid-auto-columns: minmax(0, 1fr);
    background: var(--color-bar);
    box-shadow:
      inset 0 3px 0 var(--color-bar),
      inset 0 4px 0 var(--color-brass-line);
  }
  .tabs a {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 5px;
    font-family: var(--font-label);
    font-size: 12px;
    color: var(--color-bar-text);
  }
  .tabs .mark {
    width: 8px;
    height: 8px;
    border: 1px solid var(--color-bronze);
  }
  .tabs .on .mark {
    border-color: var(--color-bar-brand);
  }
}
/* Live play is the map and nothing else: it carries its own bar, so the shell's bar, places and footer go. */
.shell--live {
  padding-bottom: 0;
}
.shell--live > .bar,
.shell--live .tabs,
.shell--live .credit {
  display: none;
}
</style>

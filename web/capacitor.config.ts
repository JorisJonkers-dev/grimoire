import type { CapacitorConfig } from '@capacitor/cli'

// The Android shell shows the hosted app when GRIMOIRE_APP_URL names it, so sign-in, the live socket
// and offline caching behave as on the web; without it the shell bundles the local build.
const url = process.env.GRIMOIRE_APP_URL

const config: CapacitorConfig = {
  appId: 'dev.jorisjonkers.grimoire',
  appName: 'Grimoire',
  webDir: 'dist',
  android: { backgroundColor: '#0E0B09' },
  ...(url ? { server: { url, cleartext: false } } : {}),
}

export default config

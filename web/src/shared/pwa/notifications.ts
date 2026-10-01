import { createPushSubscription, getPushKey } from '@/infrastructure/api/sdk.gen'

export type Subscribed = 'subscribed' | 'denied' | 'unavailable'

// pushSupported reports whether this browser can be woken by Web Push.
export function pushSupported(): boolean {
  return 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window
}

// fromBase64Url decodes a VAPID key as the Push API wants it.
export function fromBase64Url(s: string): Uint8Array<ArrayBuffer> {
  const raw = atob(s.replaceAll('-', '+').replaceAll('_', '/') + '='.repeat((4 - (s.length % 4)) % 4))
  return Uint8Array.from(raw, (c) => c.charCodeAt(0))
}

// subscribeDevice asks for this device to hear about the player's turns and Reaction Prompts.
export async function subscribeDevice(): Promise<Subscribed> {
  const key = await getPushKey()
  if (!key.data) return 'unavailable'
  if ((await Notification.requestPermission()) !== 'granted') return 'denied'
  const registration = await navigator.serviceWorker.ready
  const sub = await registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: fromBase64Url(key.data.publicKey) })
  const { endpoint = '', keys = {} } = sub.toJSON()
  const saved = await createPushSubscription({ body: { endpoint, keys: { p256dh: keys.p256dh ?? '', auth: keys.auth ?? '' } } })
  if (!saved.data) throw new Error('push subscription refused')
  return 'subscribed'
}

import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, ref } from 'vue'
import { expectAccessible } from '@/test/axe'
import { mountApp } from '@/test/mountApp'
import { jsonResponse, mountWithQuery } from '@/test/mountWithQuery'
import NotifyToggle from './NotifyToggle.vue'
import { fromBase64Url, pushSupported, subscribeDevice } from './notifications'
import { useWakeLock } from './wakeLock'

const native = vi.hoisted(() => ({ on: false, calls: [] as string[] }))
vi.mock('@capacitor/core', () => ({ Capacitor: { isNativePlatform: () => native.on } }))
vi.mock('@capacitor-community/keep-awake', () => ({
  KeepAwake: {
    keepAwake: async () => {
      native.calls.push('awake')
    },
    allowSleep: async () => {
      native.calls.push('sleep')
    },
  },
}))

function setNavigator(name: string, value: unknown) {
  Object.defineProperty(navigator, name, { value, configurable: true })
}

function removeNavigator(name: string) {
  Reflect.deleteProperty(navigator, name)
}

afterEach(() => {
  vi.unstubAllGlobals()
  for (const name of ['serviceWorker', 'wakeLock', 'onLine']) removeNavigator(name)
  native.on = false
  native.calls = []
})

describe('push notifications', () => {
  let posted: unknown[] = []
  let permission: NotificationPermission = 'granted'
  let subscription: PushSubscriptionJSON = {}
  const api = (key: number): typeof fetch =>
    vi.fn<typeof fetch>(async (input) => {
      const request = input as Request
      if (request.url.endsWith('/push/key')) return key === 200 ? jsonResponse({ publicKey: 'AQAB_-8' }) : jsonResponse({ type: 'about:blank', title: 'Not found', status: 404 }, 404)
      posted.push(await request.json())
      return jsonResponse({ id: '00000000-0000-4000-8000-000000000001' }, 201)
    })

  beforeEach(() => {
    posted = []
    permission = 'granted'
    subscription = { endpoint: 'https://push.example/1', keys: { p256dh: 'p', auth: 'a' } }
    vi.stubGlobal('PushManager', function PushManager() {})
    vi.stubGlobal('Notification', { requestPermission: async () => permission })
    setNavigator('serviceWorker', { ready: Promise.resolve({ pushManager: { subscribe: async () => ({ toJSON: () => subscription }) } }) })
  })

  it('decodes a URL-safe VAPID key and knows when push is missing', () => {
    expect([...fromBase64Url('AQAB_-8')]).toEqual([1, 0, 1, 255, 239])
    expect(pushSupported()).toBe(true)
    removeNavigator('serviceWorker')
    expect(pushSupported()).toBe(false)
  })

  it('subscribes this device, or says why not', async () => {
    mountWithQuery(NotifyToggle, api(200))
    expect(await subscribeDevice()).toBe('subscribed')
    expect(posted).toEqual([{ endpoint: 'https://push.example/1', keys: { p256dh: 'p', auth: 'a' } }])
    subscription = {}
    await expect(subscribeDevice()).rejects.toThrow('push subscription refused')
    subscription = { endpoint: 'https://push.example/2', keys: {} }
    await expect(subscribeDevice()).rejects.toThrow('push subscription refused')
    expect(posted).toHaveLength(1)
    permission = 'denied'
    expect(await subscribeDevice()).toBe('denied')
    mountWithQuery(NotifyToggle, api(404))
    expect(await subscribeDevice()).toBe('unavailable')
    mountWithQuery(NotifyToggle, () => Promise.reject(new TypeError('offline')))
    expect(await subscribeDevice()).toBe('unavailable')
  })

  it('offers the toggle and reports the outcome', async () => {
    const on = mountWithQuery(NotifyToggle, api(200))
    document.body.append(on.element as Element)
    await expectAccessible(on.element as Element)
    await on.get('[data-testid="notify-on"]').trigger('click')
    await flushPromises()
    expect(on.get('[data-testid="notify-state"]').text()).toBe('This device will hear about your turns.')
    vi.stubGlobal('Notification', { requestPermission: () => Promise.reject(new Error('blocked')) })
    const failed = mountWithQuery(NotifyToggle, api(200))
    await failed.get('[data-testid="notify-on"]').trigger('click')
    await flushPromises()
    expect(failed.get('[data-testid="notify-state"]').text()).toBe('Notifications could not be turned on. Try again shortly.')
    removeNavigator('serviceWorker')
    expect(mountWithQuery(NotifyToggle, api(200)).find('[data-testid="notify"]').exists()).toBe(false)
  })
})

describe('offline state', () => {
  it('tells the player when the device goes offline and back', async () => {
    const { wrapper } = await mountApp('/compendium/spells', {})
    expect(wrapper.find('[data-testid="offline"]').exists()).toBe(false)
    setNavigator('onLine', false)
    window.dispatchEvent(new Event('offline'))
    await flushPromises()
    expect(wrapper.get('[data-testid="offline"]').text()).toContain('You are offline.')
    setNavigator('onLine', true)
    window.dispatchEvent(new Event('online'))
    await flushPromises()
    expect(wrapper.find('[data-testid="offline"]').exists()).toBe(false)
  })
})

describe('wake lock', () => {
  class Sentinel extends EventTarget {
    released = false
    async release() {
      this.released = true
    }
  }
  let held: Sentinel[] = []
  let refuse = false
  let visibility: DocumentVisibilityState = 'visible'

  beforeEach(() => {
    held = []
    refuse = false
    visibility = 'visible'
    vi.spyOn(document, 'visibilityState', 'get').mockImplementation(() => visibility)
    setNavigator('wakeLock', {
      request: async () => {
        if (refuse) throw new Error('not allowed')
        const s = new Sentinel()
        held.push(s)
        return s
      },
    })
  })

  function host(active: { value: boolean }) {
    return mount(
      defineComponent({
        setup() {
          useWakeLock(() => active.value)
          return () => h('p')
        },
      }),
    )
  }

  it('holds the screen while live and visible, and takes it again on return', async () => {
    const active = ref(false)
    const w = host(active)
    await flushPromises()
    expect(held).toHaveLength(0)
    active.value = true
    await flushPromises()
    expect(held).toHaveLength(1)
    visibility = 'hidden'
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(held[0]?.released).toBe(true)
    visibility = 'visible'
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(held).toHaveLength(2)
    held[1]?.dispatchEvent(new Event('release'))
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(held).toHaveLength(3)
    w.unmount()
    await flushPromises()
    expect(held[2]?.released).toBe(true)
  })

  it('does without a lock when the browser has none or refuses', async () => {
    refuse = true
    const refused = host(ref(true))
    await flushPromises()
    expect(held).toHaveLength(0)
    refused.unmount()
    removeNavigator('wakeLock')
    host(ref(true)).unmount()
    await flushPromises()
    expect(held).toHaveLength(0)
  })

  it('keeps the Android shell awake natively', async () => {
    native.on = true
    const active = ref(true)
    host(active)
    await flushPromises()
    active.value = false
    await flushPromises()
    expect(native.calls).toEqual(['awake', 'sleep'])
  })
})

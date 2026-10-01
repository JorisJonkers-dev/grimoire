import { onBeforeUnmount, ref, type Ref } from 'vue'

// useOnline follows whether the device has a network connection.
export function useOnline(): Ref<boolean> {
  const online = ref(navigator.onLine)
  const update = () => {
    online.value = navigator.onLine
  }
  window.addEventListener('online', update)
  window.addEventListener('offline', update)
  onBeforeUnmount(() => {
    window.removeEventListener('online', update)
    window.removeEventListener('offline', update)
  })
  return online
}

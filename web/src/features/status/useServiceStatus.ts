import { useQuery } from '@tanstack/vue-query'
import { getStatusOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'

export function useServiceStatus() {
  return useQuery({ ...getStatusOptions(), retry: false })
}

import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { fetchJSON } from '../lib/api'
import type { ResourceConfig } from '../resource-config'
import type { GenericResourceItem } from '../types/generic-resource'

export function useGenericResourceDetail(config: ResourceConfig) {
  const route = useRoute()
  const loading = ref(false)
  const error = ref<string | null>(null)
  const item = ref<GenericResourceItem | null>(null)

  const encodedName = computed(() => encodeURIComponent(String(route.params.name || '')))
  const displayName = computed(() => decodeURIComponent(String(route.params.name || '')))

  async function load() {
    if (!route.params.name) {
      item.value = null
      error.value = 'Missing resource name.'
      return
    }

    loading.value = true
    error.value = null

    try {
      item.value = await fetchJSON<GenericResourceItem>(`${config.apiPath}/${encodedName.value}`)
    } catch (err) {
      error.value = err instanceof Error ? err.message : `Unable to load ${config.title} detail.`
      item.value = null
    } finally {
      loading.value = false
    }
  }

  watch(() => route.params.name, load, { immediate: true })

  return {
    loading,
    error,
    item,
    displayName,
  }
}

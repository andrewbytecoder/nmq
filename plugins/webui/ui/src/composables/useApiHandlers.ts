import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { fetchPaginatedJSON } from '../lib/api'
import type { ApiHandlerItem } from '../types/api-handler'

type HandlerProtocol = 'routers'
type SortDirection = 'asc' | 'desc'
type ApiHandlerPayload = ApiHandlerItem & {
  Method?: string
  Path?: string
  Handler?: string
}

const defaultPageSize = 10

function queryStringValue(value: unknown) {
  return typeof value === 'string' ? value : ''
}

function endpointFor(protocol: HandlerProtocol) {
  return protocol === 'routers' ? '/handlers/routers' : '/handlers/routers'
}

function normalizeApiHandlerItem(item: ApiHandlerPayload): ApiHandlerItem {
  return {
    method: item.method ?? item.Method ?? '',
    path: item.path ?? item.Path ?? '',
    handler: item.handler ?? item.Handler ?? '',
  }
}

export function useApiHandlers(protocol: HandlerProtocol) {
  const route = useRoute()
  const router = useRouter()

  const loading = ref(false)
  const error = ref<string | null>(null)
  const items = ref<ApiHandlerItem[]>([])
  const nextPage = ref(1)
  const totalPages = ref(1)

  const search = computed({
    get: () => queryStringValue(route.query.search),
    set: (value: string) => {
      void router.replace({
        query: {
          ...route.query,
          search: value || undefined,
          page: undefined,
        },
      })
    },
  })

  const method = computed({
    get: () => queryStringValue(route.query.method),
    set: (value: string) => {
      void router.replace({
        query: {
          ...route.query,
          method: value || undefined,
          page: undefined,
        },
      })
    },
  })

  const sortBy = computed({
    get: () => queryStringValue(route.query.sortBy) || 'path',
    set: (value: string) => {
      void router.replace({
        query: {
          ...route.query,
          sortBy: value,
          page: undefined,
        },
      })
    },
  })

  const direction = computed<SortDirection>({
    get: () => (queryStringValue(route.query.direction) === 'desc' ? 'desc' : 'asc'),
    set: (value: SortDirection) => {
      void router.replace({
        query: {
          ...route.query,
          direction: value,
          page: undefined,
        },
      })
    },
  })

  const currentPage = computed({
    get: () => Number(queryStringValue(route.query.page) || '1'),
    set: (value: number) => {
      void router.replace({
        query: {
          ...route.query,
          page: value > 1 ? String(value) : undefined,
        },
      })
    },
  })

  const pageCount = computed(() => {
    return Math.max(totalPages.value, 1)
  })

  const canGoNext = computed(() => nextPage.value !== 1)
  const canGoPrevious = computed(() => currentPage.value > 1)

  async function load() {
    loading.value = true
    error.value = null

    const params = new URLSearchParams({
      page: String(currentPage.value),
      per_page: String(defaultPageSize),
      sortBy: sortBy.value,
      direction: direction.value,
    })

    if (search.value) {
      params.set('search', search.value)
    }

    if (method.value) {
      params.set('method', method.value)
    }

    try {
      const response = await fetchPaginatedJSON<ApiHandlerPayload>(`${endpointFor(protocol)}?${params.toString()}`)
      items.value = response.data.map(normalizeApiHandlerItem)
      nextPage.value = response.nextPage
      totalPages.value = response.totalPages
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Unable to load API handlers.'
      items.value = []
      nextPage.value = 1
      totalPages.value = 1
    } finally {
      loading.value = false
    }
  }

  watch(
    () => [route.query.search, route.query.method, route.query.sortBy, route.query.direction, route.query.page],
    load,
    { immediate: true },
  )

  return {
    loading,
    error,
    items,
    search,
    method,
    sortBy,
    direction,
    currentPage,
    pageCount,
    canGoNext,
    canGoPrevious,
  }
}

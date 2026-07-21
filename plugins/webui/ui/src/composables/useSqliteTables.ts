import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { fetchPaginatedJSON } from '../lib/api'
import type { SqliteTableItem } from '../types/sqlite-table'

type SortDirection = 'asc' | 'desc'

const defaultPageSize = 10

function queryStringValue(value: unknown) {
  return typeof value === 'string' ? value : ''
}

export function useSqliteTables() {
  const route = useRoute()
  const router = useRouter()

  const loading = ref(false)
  const error = ref<string | null>(null)
  const items = ref<SqliteTableItem[]>([])
  const nextPage = ref(1)

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

  const sortBy = computed({
    get: () => queryStringValue(route.query.sortBy) || 'name',
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
    if (nextPage.value === 1) {
      return currentPage.value
    }

    return Math.max(currentPage.value + 1, 2)
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

    try {
      const response = await fetchPaginatedJSON<SqliteTableItem>(`/storage/sqlite?${params.toString()}`)
      items.value = response.data
      nextPage.value = response.nextPage
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Unable to load SQLite tables.'
      items.value = []
      nextPage.value = 1
    } finally {
      loading.value = false
    }
  }

  watch(() => [route.query.search, route.query.sortBy, route.query.direction, route.query.page], load, { immediate: true })

  return {
    loading,
    error,
    items,
    search,
    sortBy,
    direction,
    currentPage,
    pageCount,
    canGoNext,
    canGoPrevious,
  }
}

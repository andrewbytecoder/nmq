import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { fetchJSON } from '../lib/api'
import type { SqliteTableDetail } from '../types/sqlite-table'

const defaultPageSize = 20

function queryStringValue(value: unknown) {
  return typeof value === 'string' ? value : ''
}

export function useSqliteTableDetail() {
  const route = useRoute()
  const router = useRouter()

  const loading = ref(false)
  const error = ref<string | null>(null)
  const detail = ref<SqliteTableDetail | null>(null)

  const encodedTableName = computed(() => encodeURIComponent(String(route.params.tableName || '')))
  const displayName = computed(() => decodeURIComponent(String(route.params.tableName || '')))

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
    if (!detail.value) {
      return 1
    }

    return Math.max(Math.ceil(detail.value.rowCount / detail.value.perPage), 1)
  })

  const canGoNext = computed(() => currentPage.value < pageCount.value)
  const canGoPrevious = computed(() => currentPage.value > 1)

  async function load() {
    if (!route.params.tableName) {
      detail.value = null
      error.value = 'Missing table name.'
      return
    }

    loading.value = true
    error.value = null

    try {
      detail.value = await fetchJSON<SqliteTableDetail>(
        `/storage/sqlite/${encodedTableName.value}?page=${currentPage.value}&per_page=${defaultPageSize}`,
      )
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Unable to load SQLite table detail.'
      detail.value = null
    } finally {
      loading.value = false
    }
  }

  watch(() => [route.params.tableName, route.query.page], load, { immediate: true })

  return {
    loading,
    error,
    detail,
    displayName,
    currentPage,
    pageCount,
    canGoNext,
    canGoPrevious,
  }
}

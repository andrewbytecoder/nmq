import { computed, onMounted, ref } from 'vue'

import { fetchJSON } from '../lib/api'
import type { DashboardOverview, EntryPointInfo, ProtocolOverview, ResourceStats } from '../types/dashboard'

const statKeys = ['routers', 'services', 'middlewares'] as const

function hasUsefulStats(section?: ProtocolOverview, expected: number = statKeys.length) {
  if (!section) {
    return false
  }

  const values = Object.values(section).filter(Boolean) as ResourceStats[]
  if (!values.length) {
    return false
  }

  const emptyCount = values.filter((value) => !value.errors && !value.warnings && !value.total).length
  return emptyCount !== expected
}

export function useDashboardOverview() {
  const loading = ref(true)
  const error = ref<string | null>(null)
  const entrypoints = ref<EntryPointInfo[]>([])
  const overview = ref<DashboardOverview | null>(null)

  async function load() {
    loading.value = true
    error.value = null

    try {
      const [nextEntrypoints, nextOverview] = await Promise.all([
        fetchJSON<EntryPointInfo[]>('/entrypoints'),
        fetchJSON<DashboardOverview>('/overview'),
      ])

      entrypoints.value = nextEntrypoints
      overview.value = nextOverview
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Unable to load dashboard data.'
      entrypoints.value = []
      overview.value = null
    } finally {
      loading.value = false
    }
  }

  onMounted(load)

  const features = computed(() =>
    overview.value?.features
      ? Object.entries(overview.value.features).map(([name, value]) => ({
          name,
          value,
        }))
      : [],
  )

  const resourceGroups = computed(() => [
    {
      key: 'http',
      title: 'HTTP',
      routePrefix: '/http',
      hasData: hasUsefulStats(overview.value?.http),
      stats: overview.value?.http ?? {},
    },
    {
      key: 'tcp',
      title: 'TCP',
      routePrefix: '/tcp',
      hasData: hasUsefulStats(overview.value?.tcp),
      stats: overview.value?.tcp ?? {},
    },
    {
      key: 'udp',
      title: 'UDP',
      routePrefix: '/udp',
      hasData: hasUsefulStats(overview.value?.udp, 2),
      stats: overview.value?.udp ?? {},
    },
  ])

  return {
    loading,
    error,
    entrypoints,
    overview,
    features,
    resourceGroups,
    reload: load,
  }
}

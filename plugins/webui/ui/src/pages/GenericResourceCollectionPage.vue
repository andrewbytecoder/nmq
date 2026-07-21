<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import { useGenericResourceCollection } from '../composables/useGenericResourceCollection'
import { getResourceConfig } from '../resource-config'

const props = defineProps<{
  resourceKey: string
}>()

const route = useRoute()
const config = getResourceConfig(props.resourceKey)

if (!config) {
  throw new Error(`Unknown resource config: ${props.resourceKey}`)
}

const {
  canGoNext,
  canGoPrevious,
  currentPage,
  direction,
  error,
  items,
  loading,
  pageCount,
  search,
  sortBy,
  status,
} = useGenericResourceCollection(config)

const statusOptions = [
  { title: 'All statuses', value: '' },
  { title: 'Enabled', value: 'enabled' },
  { title: 'Disabled', value: 'disabled' },
  { title: 'Warning', value: 'warning' },
  { title: 'Expired', value: 'expired' },
]

const sortOptions = computed(() =>
  config.columns
    .filter((column) => !['tls'].includes(column.type))
    .map((column) => ({
      title: column.label,
      value: column.key === 'servers' ? 'servers' : column.key,
    })),
)

const providerIcons: Record<string, string> = {
  docker: 'mdi-docker',
  file: 'mdi-file-document-outline',
  kubernetescrd: 'mdi-kubernetes',
  internal: 'mdi-shield-outline',
}

const statusColor = computed(() => (value: string) => {
  switch (value) {
    case 'enabled':
      return 'success'
    case 'disabled':
      return 'error'
    case 'warning':
      return 'warning'
    case 'expired':
      return 'error'
    default:
      return 'secondary'
  }
})

function providerIcon(name: string) {
  return providerIcons[String(name).toLowerCase()] ?? 'mdi-cloud-outline'
}

function toggleDirection() {
  direction.value = direction.value === 'asc' ? 'desc' : 'asc'
}

function detailLink(name: unknown) {
  return `${config.routePath}/${encodeURIComponent(String(name || ''))}`
}

function getValue(item: Record<string, unknown>, key: string) {
  if (key === 'servers') {
    const loadBalancer = item.loadBalancer as { servers?: unknown[] } | undefined
    return loadBalancer?.servers?.length ?? 0
  }

  if (key === 'using') {
    return (item.using as unknown[]) || (item.entryPoints as unknown[]) || []
  }

  if (key === 'priority') {
    return item.priorityStr || item.priority || '-'
  }

  return item[key]
}

function formatDate(value: unknown) {
  if (typeof value !== 'string' || !value) {
    return '-'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleString('zh-CN', { hour12: false })
}
</script>

<template>
  <div class="page-shell" :data-testid="`${config.key}-page`">
    <div class="page-header">
      <div>
        <div class="eyebrow">{{ config.section }}</div>
        <h1 class="page-title">{{ config.title }}</h1>
        <p class="page-description">{{ config.description }}</p>
      </div>
      <v-chip color="success" variant="tonal" prepend-icon="mdi-view-list-outline">
        Vue collection page active
      </v-chip>
    </div>

    <v-card rounded="xl" variant="outlined">
      <v-card-text>
        <v-row dense>
          <v-col cols="12" md="5">
            <v-text-field
              v-model="search"
              clearable
              :label="`Search ${config.title.toLowerCase()}`"
              prepend-inner-icon="mdi-magnify"
              variant="outlined"
              density="comfortable"
              hide-details
            />
          </v-col>
          <v-col cols="12" md="3">
            <v-select
              v-model="status"
              :items="statusOptions"
              label="Status"
              variant="outlined"
              density="comfortable"
              hide-details
            />
          </v-col>
          <v-col cols="12" md="3">
            <v-select
              v-model="sortBy"
              :items="sortOptions"
              label="Sort by"
              variant="outlined"
              density="comfortable"
              hide-details
            />
          </v-col>
          <v-col cols="12" md="1">
            <v-btn
              block
              height="56"
              variant="outlined"
              :prepend-icon="direction === 'asc' ? 'mdi-arrow-up' : 'mdi-arrow-down'"
              @click="toggleDirection"
            >
              {{ direction }}
            </v-btn>
          </v-col>
        </v-row>
      </v-card-text>
    </v-card>

    <v-alert
      v-if="error"
      type="warning"
      variant="tonal"
      border="start"
      :title="`Failed to fetch ${config.title}`"
      :text="error"
    />

    <v-table class="resource-table" density="comfortable">
      <thead>
        <tr>
          <th v-for="column in config.columns" :key="column.key">{{ column.label }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="loading">
          <td :colspan="config.columns.length">
            <v-skeleton-loader :type="`table-row-divider@${config.columns.length}`" />
          </td>
        </tr>
        <tr v-for="item in items" :key="String(item.name || route.fullPath)">
          <td v-for="column in config.columns" :key="column.key">
            <template v-if="column.type === 'status'">
              <v-chip :color="statusColor(String(getValue(item, column.key) || ''))" size="small" variant="tonal">
                {{ getValue(item, column.key) || '-' }}
              </v-chip>
            </template>
            <template v-else-if="column.type === 'tls'">
              <v-icon
                :icon="getValue(item, column.key) ? 'mdi-lock-outline' : 'mdi-lock-open-variant-outline'"
                :color="getValue(item, column.key) ? 'success' : 'disabled'"
              />
            </template>
            <template v-else-if="column.type === 'chips'">
              <div class="chip-wrap">
                <v-chip
                  v-for="entry in (getValue(item, column.key) as unknown[])"
                  :key="`${String(item.name)}-${String(entry)}`"
                  size="x-small"
                  variant="outlined"
                >
                  {{ String(entry) }}
                </v-chip>
                <span v-if="!((getValue(item, column.key) as unknown[]) || []).length">-</span>
              </div>
            </template>
            <template v-else-if="column.type === 'provider'">
              <div class="provider-pill">
                <v-icon :icon="providerIcon(String(getValue(item, column.key) || ''))" size="18" />
                <span>{{ getValue(item, column.key) || '-' }}</span>
              </div>
            </template>
            <template v-else-if="column.type === 'date'">
              {{ formatDate(getValue(item, column.key)) }}
            </template>
            <template v-else-if="column.type === 'array-count'">
              {{ Array.isArray(getValue(item, column.key)) ? (getValue(item, column.key) as unknown[]).length : 0 }}
            </template>
            <template v-else-if="column.key === 'name'">
              <router-link class="name-link" :to="detailLink(item.name)">
                {{ getValue(item, column.key) || '-' }}
              </router-link>
            </template>
            <template v-else>
              <span class="text-value">{{ getValue(item, column.key) || '-' }}</span>
            </template>
          </td>
        </tr>
        <tr v-if="!loading && !items.length">
          <td :colspan="config.columns.length" class="empty-state">No data available.</td>
        </tr>
      </tbody>
    </v-table>

    <div class="pagination-row">
      <div class="page-meta">Page {{ currentPage }} of {{ pageCount }}</div>
      <div class="page-actions">
        <v-btn :disabled="!canGoPrevious" variant="outlined" @click="currentPage = currentPage - 1">
          Previous
        </v-btn>
        <v-btn :disabled="!canGoNext" color="primary" variant="flat" @click="currentPage = currentPage + 1">
          Next
        </v-btn>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page-shell {
  display: grid;
  gap: 1.25rem;
}

.page-header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: flex-start;
}

.eyebrow {
  margin-bottom: 0.5rem;
  color: #1f8aa8;
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.page-title {
  margin: 0;
  color: #16324f;
  font-size: clamp(1.9rem, 3vw, 2.8rem);
}

.page-description {
  max-width: 56rem;
  margin: 0.85rem 0 0;
  color: #54667a;
  line-height: 1.7;
}

.resource-table :deep(th) {
  color: #5c6d7d;
  font-weight: 700;
}

.resource-table :deep(td) {
  vertical-align: top;
}

.chip-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.provider-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  color: #42576b;
  text-transform: capitalize;
}

.name-link {
  color: #1f8aa8;
  font-weight: 600;
  text-decoration: none;
}

.name-link:hover {
  text-decoration: underline;
}

.text-value {
  color: #33495c;
  word-break: break-word;
}

.empty-state {
  color: #66788a;
  text-align: center;
}

.pagination-row {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: center;
}

.page-meta {
  color: #66788a;
}

.page-actions {
  display: flex;
  gap: 0.75rem;
}

@media (max-width: 960px) {
  .page-header,
  .pagination-row {
    flex-direction: column;
    align-items: stretch;
  }

  .page-actions {
    width: 100%;
  }
}
</style>

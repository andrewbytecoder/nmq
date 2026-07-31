<script setup lang="ts">
import { computed } from 'vue'

import { useApiHandlers } from '../../composables/useApiHandlers'

const props = defineProps<{
  protocol: 'routers'
}>()

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
  method,
  sortBy,
} = useApiHandlers(props.protocol)

const methodOptions = [
  { title: 'All methods', value: '' },
  { title: 'GET', value: 'GET' },
  { title: 'POST', value: 'POST' },
  { title: 'PUT', value: 'PUT' },
  { title: 'PATCH', value: 'PATCH' },
  { title: 'DELETE', value: 'DELETE' },
  { title: 'OPTIONS', value: 'OPTIONS' },
  { title: 'HEAD', value: 'HEAD' },
]

const sortOptions = [
  { title: 'Path', value: 'path' },
  { title: 'Method', value: 'method' },
  { title: 'Handler', value: 'handler' },
]

const protocolLabel = computed(() => (props.protocol === 'routers' ? 'ROUTERS' : props.protocol.toUpperCase()))
const pageTitle = computed(() => `${protocolLabel.value} Handler`)
const endpointLabel = computed(() => `/api/handlers/${props.protocol}`)

const methodColor = computed(() => (value: string) => {
  switch (value.toUpperCase()) {
    case 'GET':
      return 'success'
    case 'POST':
      return 'primary'
    case 'PUT':
      return 'info'
    case 'PATCH':
      return 'warning'
    case 'DELETE':
      return 'error'
    default:
      return 'secondary'
  }
})

function toggleDirection() {
  direction.value = direction.value === 'asc' ? 'desc' : 'asc'
}
</script>

<template>
  <div class="page-shell" :data-testid="`${props.protocol}-handlers-page`">
    <div class="page-header">
      <div>
        <div class="eyebrow">API</div>
        <h1 class="page-title">{{ pageTitle }}</h1>
        <p class="page-description">
          Browse {{ protocolLabel }} request handlers exposed by dpproxy. This page keeps the same search,
          method filtering, sorting, and paging workflow as the other dashboard collection views, and reads
          from {{ endpointLabel }}.
        </p>
      </div>
      <v-chip color="primary" variant="tonal" prepend-icon="mdi-api">
        Route catalog
      </v-chip>
    </div>

    <v-card rounded="xl" variant="outlined">
      <v-card-text>
        <v-row dense>
          <v-col cols="12" md="5">
            <v-text-field
              v-model="search"
              clearable
              label="Search path or handler"
              prepend-inner-icon="mdi-magnify"
              variant="outlined"
              density="comfortable"
              hide-details
            />
          </v-col>
          <v-col cols="12" md="3">
            <v-select
              v-model="method"
              :items="methodOptions"
              label="Method"
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
      :title="`Failed to fetch ${pageTitle}`"
      :text="error"
    />

    <v-table class="handler-table" density="comfortable">
      <thead>
        <tr>
          <th>Method</th>
          <th>Path</th>
          <th>Handler</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="loading">
          <td colspan="3">
            <v-skeleton-loader type="table-row-divider@4" />
          </td>
        </tr>
        <tr v-for="item in items" :key="`${item.method}-${item.path}-${item.handler}`">
          <td>
            <v-chip :color="methodColor(item.method)" size="small" variant="tonal">
              {{ item.method }}
            </v-chip>
          </td>
          <td class="path-cell">{{ item.path }}</td>
          <td class="handler-cell">{{ item.handler }}</td>
        </tr>
        <tr v-if="!loading && !items.length">
          <td colspan="3" class="empty-state">No data available.</td>
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

.handler-table :deep(th) {
  color: #5c6d7d;
  font-weight: 700;
}

.handler-table :deep(td) {
  vertical-align: top;
}

.path-cell,
.handler-cell {
  color: #33495c;
  line-height: 1.5;
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

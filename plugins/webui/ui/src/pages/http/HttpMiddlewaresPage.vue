<script setup lang="ts">
import { computed } from 'vue'

import { useGenericResourceCollection } from '../../composables/useGenericResourceCollection'
import { parseMiddlewareType } from '../../libs/parsers'
import { getResourceConfig } from '../../resource-config'

const config = getResourceConfig('http-middlewares')

if (!config) {
  throw new Error('Missing resource config for http-middlewares')
}

const { canGoNext, canGoPrevious, currentPage, direction, error, items, loading, pageCount, search, sortBy, status } =
  useGenericResourceCollection(config)

const statusOptions = [
  { title: 'All statuses', value: '' },
  { title: 'Enabled', value: 'enabled' },
  { title: 'Disabled', value: 'disabled' },
  { title: 'Warning', value: 'warning' },
]

const sortOptions = [
  { title: 'Name', value: 'name' },
  { title: 'Type', value: 'type' },
  { title: 'Provider', value: 'provider' },
  { title: 'Status', value: 'status' },
]

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
    default:
      return 'secondary'
  }
})

function providerIcon(name: string) {
  return providerIcons[name.toLowerCase()] ?? 'mdi-cloud-outline'
}

function toggleDirection() {
  direction.value = direction.value === 'asc' ? 'desc' : 'asc'
}

function middlewareType(item: Record<string, unknown>) {
  return parseMiddlewareType(item) || '-'
}
</script>

<template>
  <div class="page-shell" data-testid="http-middlewares-page">
    <div class="page-header">
      <div>
        <div class="eyebrow">HTTP</div>
        <h1 class="page-title">HTTP Middlewares</h1>
        <p class="page-description">
          Browse HTTP middlewares with the new Vue stack, including middleware type, provider origin, and
          direct navigation into the specialized middleware detail page.
        </p>
      </div>
      <v-chip color="success" variant="tonal" prepend-icon="mdi-tune-variant">
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
              label="Search middleware name"
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
      title="Failed to fetch HTTP middlewares"
      :text="error"
    />

    <v-table class="middleware-table" density="comfortable">
      <thead>
        <tr>
          <th>Status</th>
          <th>Name</th>
          <th>Type</th>
          <th>Provider</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="loading">
          <td colspan="4">
            <v-skeleton-loader type="table-row-divider@4" />
          </td>
        </tr>
        <tr v-for="middleware in items" :key="String(middleware.name)">
          <td>
            <v-chip :color="statusColor(String(middleware.status || ''))" size="small" variant="tonal">
              {{ middleware.status || '-' }}
            </v-chip>
          </td>
          <td>
            <router-link class="name-link" :to="`/http/middlewares/${encodeURIComponent(String(middleware.name || ''))}`">
              {{ middleware.name || '-' }}
            </router-link>
          </td>
          <td class="type-cell">{{ middlewareType(middleware) }}</td>
          <td>
            <div class="provider-pill">
              <v-icon :icon="providerIcon(String(middleware.provider || ''))" size="18" />
              <span>{{ middleware.provider || '-' }}</span>
            </div>
          </td>
        </tr>
        <tr v-if="!loading && !items.length">
          <td colspan="4" class="empty-state">No data available.</td>
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
.page-shell { display: grid; gap: 1.25rem; }
.page-header { display: flex; justify-content: space-between; gap: 1rem; align-items: flex-start; }
.eyebrow { margin-bottom: 0.5rem; color: #1f8aa8; font-size: 0.8rem; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase; }
.page-title { margin: 0; color: #16324f; font-size: clamp(1.9rem, 3vw, 2.8rem); }
.page-description { max-width: 56rem; margin: 0.85rem 0 0; color: #54667a; line-height: 1.7; }
.middleware-table :deep(th) { color: #5c6d7d; font-weight: 700; }
.middleware-table :deep(td) { vertical-align: top; }
.type-cell { color: #33495c; text-transform: lowercase; }
.name-link { color: #1f8aa8; font-weight: 600; text-decoration: none; }
.name-link:hover { text-decoration: underline; }
.provider-pill { display: inline-flex; align-items: center; gap: 0.4rem; color: #42576b; text-transform: capitalize; }
.empty-state { color: #66788a; text-align: center; }
.pagination-row { display: flex; justify-content: space-between; gap: 1rem; align-items: center; }
.page-meta { color: #66788a; }
.page-actions { display: flex; gap: 0.75rem; }
@media (max-width: 960px) { .page-header, .pagination-row { flex-direction: column; align-items: stretch; } .page-actions { width: 100%; } }
</style>

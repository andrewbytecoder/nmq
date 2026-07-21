<script setup lang="ts">
import { computed } from 'vue'

import { useHttpRouters } from '../../composables/useHttpRouters'

const providerIcons: Record<string, string> = {
  docker: 'mdi-docker',
  file: 'mdi-file-document-outline',
  kubernetescrd: 'mdi-kubernetes',
  internal: 'mdi-shield-outline',
}

const { canGoNext, canGoPrevious, currentPage, direction, error, items, loading, pageCount, search, sortBy, status } =
  useHttpRouters()

const statusOptions = [
  { title: 'All statuses', value: '' },
  { title: 'Enabled', value: 'enabled' },
  { title: 'Disabled', value: 'disabled' },
  { title: 'Warning', value: 'warning' },
]

const sortOptions = [
  { title: 'Name', value: 'name' },
  { title: 'Service', value: 'service' },
  { title: 'Provider', value: 'provider' },
  { title: 'Status', value: 'status' },
  { title: 'Rule', value: 'rule' },
  { title: 'Priority', value: 'priority' },
]

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
</script>

<template>
  <div class="page-shell" data-testid="http-routers-page">
    <div class="page-header">
      <div>
        <div class="eyebrow">HTTP</div>
        <h1 class="page-title">HTTP Routers</h1>
        <p class="page-description">
          This page is now backed by the Vue stack and talks to the real paginated `/api/http/routers`
          endpoint. Search, status filtering, sorting, and page navigation are already carried over for the
          first migrated collection view.
        </p>
      </div>
      <v-chip color="success" variant="tonal" prepend-icon="mdi-check-decagram-outline">
        Vue list page active
      </v-chip>
    </div>

    <v-card rounded="xl" variant="outlined">
      <v-card-text>
        <v-row dense>
          <v-col cols="12" md="5">
            <v-text-field
              v-model="search"
              clearable
              label="Search router name"
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
      title="Failed to fetch HTTP routers"
      :text="error"
    />

    <v-table class="router-table" density="comfortable">
      <thead>
        <tr>
          <th>Status</th>
          <th>TLS</th>
          <th>Rule</th>
          <th>Entrypoints</th>
          <th>Name</th>
          <th>Service</th>
          <th>Provider</th>
          <th>Priority</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="loading">
          <td colspan="8">
            <v-skeleton-loader type="table-row-divider@6" />
          </td>
        </tr>
        <tr v-for="router in items" :key="router.name">
          <td>
            <v-chip :color="statusColor(router.status)" size="small" variant="tonal">
              {{ router.status }}
            </v-chip>
          </td>
          <td>
            <v-icon
              :icon="router.tls ? 'mdi-lock-outline' : 'mdi-lock-open-variant-outline'"
              :color="router.tls ? 'success' : 'disabled'"
            />
          </td>
          <td class="rule-cell">{{ router.rule || '-' }}</td>
          <td>
            <div class="chip-wrap">
              <v-chip
                v-for="entrypoint in router.using || router.entryPoints || []"
                :key="`${router.name}-${entrypoint}`"
                size="x-small"
                variant="outlined"
              >
                {{ entrypoint }}
              </v-chip>
              <span v-if="!(router.using || router.entryPoints)?.length">-</span>
            </div>
          </td>
          <td>
            <router-link class="name-link" :to="`/http/routers/${encodeURIComponent(router.name)}`">
              {{ router.name }}
            </router-link>
          </td>
          <td>{{ router.service || '-' }}</td>
          <td>
            <div class="provider-pill">
              <v-icon :icon="providerIcon(router.provider)" size="18" />
              <span>{{ router.provider }}</span>
            </div>
          </td>
          <td>{{ router.priorityStr || router.priority || '-' }}</td>
        </tr>
        <tr v-if="!loading && !items.length">
          <td colspan="8" class="empty-state">No data available.</td>
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

.router-table :deep(th) {
  color: #5c6d7d;
  font-weight: 700;
}

.router-table :deep(td) {
  vertical-align: top;
}

.rule-cell {
  max-width: 25rem;
  color: #33495c;
  line-height: 1.5;
  word-break: break-word;
}

.chip-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.name-link {
  color: #1f8aa8;
  font-weight: 600;
  text-decoration: none;
}

.name-link:hover {
  text-decoration: underline;
}

.provider-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  color: #42576b;
  text-transform: capitalize;
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

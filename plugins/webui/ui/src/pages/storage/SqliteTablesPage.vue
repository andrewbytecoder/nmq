<script setup lang="ts">
import { computed } from 'vue'

import { useSqliteTables } from '../../composables/useSqliteTables'

const { canGoNext, canGoPrevious, currentPage, direction, error, items, loading, pageCount, search, sortBy } =
  useSqliteTables()

const sortOptions = [
  { title: 'Name', value: 'name' },
  { title: 'Rows', value: 'rowCount' },
  { title: 'Columns', value: 'columnCount' },
  { title: 'Primary Keys', value: 'primaryCount' },
  { title: 'Type', value: 'type' },
]

const tableCountLabel = computed(() => `${items.value.length} table${items.value.length === 1 ? '' : 's'} on this page`)

function toggleDirection() {
  direction.value = direction.value === 'asc' ? 'desc' : 'asc'
}
</script>

<template>
  <div class="page-shell" data-testid="storage-sqlite-page">
    <div class="page-header">
      <div>
        <div class="eyebrow">STORAGE</div>
        <h1 class="page-title">SQLITE</h1>
        <p class="page-description">
          Inspect the SQLite tables used by the dpcore plugin. This view keeps the same search, sorting, and paging
          rhythm as the HTTP Routers screen while focusing on table inventory and key metadata.
        </p>
      </div>
      <v-chip color="success" variant="tonal" prepend-icon="mdi-database-outline">
        {{ tableCountLabel }}
      </v-chip>
    </div>

    <v-card rounded="xl" variant="outlined">
      <v-card-text>
        <v-row dense>
          <v-col cols="12" md="6">
            <v-text-field
              v-model="search"
              clearable
              label="Search table name"
              prepend-inner-icon="mdi-magnify"
              variant="outlined"
              density="comfortable"
              hide-details
            />
          </v-col>
          <v-col cols="12" md="5">
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
      title="Failed to fetch SQLite tables"
      :text="error"
    />

    <v-table class="resource-table" density="comfortable">
      <thead>
        <tr>
          <th>Name</th>
          <th>Type</th>
          <th>Rows</th>
          <th>Columns</th>
          <th>Primary Keys</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="loading">
          <td colspan="5">
            <v-skeleton-loader type="table-row-divider@5" />
          </td>
        </tr>
        <tr v-for="table in items" :key="table.name">
          <td class="name-cell">
            <RouterLink class="table-link" :to="{ name: 'storage-sqlite-detail', params: { tableName: table.name } }">
              {{ table.name }}
            </RouterLink>
          </td>
          <td class="text-value">{{ table.type || '-' }}</td>
          <td class="text-value">{{ table.rowCount }}</td>
          <td class="text-value">{{ table.columnCount }}</td>
          <td>
            <div class="chip-wrap">
              <v-chip
                v-for="primaryKey in table.primaryKeys"
                :key="`${table.name}-${primaryKey}`"
                size="x-small"
                variant="outlined"
              >
                {{ primaryKey }}
              </v-chip>
              <span v-if="!table.primaryKeys.length">-</span>
            </div>
          </td>
        </tr>
        <tr v-if="!loading && !items.length">
          <td colspan="5" class="empty-state">No data available.</td>
        </tr>
      </tbody>
    </v-table>

    <div class="pagination-row">
      <div class="page-meta">Page {{ currentPage }} of {{ pageCount }}</div>
      <div class="page-actions">
        <v-btn :disabled="!canGoPrevious" variant="outlined" @click="currentPage = currentPage - 1">PREVIOUS</v-btn>
        <v-btn :disabled="!canGoNext" color="primary" variant="flat" @click="currentPage = currentPage + 1">NEXT</v-btn>
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

.name-cell {
  font-weight: 600;
}

.table-link {
  color: #1f8aa8;
  text-decoration: none;
}

.table-link:hover {
  text-decoration: underline;
}

.chip-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
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

<script setup lang="ts">
import { computed } from 'vue'

import { useSqliteTableDetail } from '../../composables/useSqliteTableDetail'

const { canGoNext, canGoPrevious, currentPage, detail, displayName, error, loading, pageCount } = useSqliteTableDetail()

const rowCountLabel = computed(() => {
  const rowCount = detail.value?.rowCount ?? 0
  return `${rowCount} row${rowCount === 1 ? '' : 's'}`
})

const columnCountLabel = computed(() => {
  const columnCount = detail.value?.columns.length ?? 0
  return `${columnCount} column${columnCount === 1 ? '' : 's'}`
})

function formatCellValue(value: unknown) {
  if (value === null || value === undefined || value === '') {
    return '-'
  }

  if (typeof value === 'object') {
    try {
      return JSON.stringify(value)
    } catch {
      return String(value)
    }
  }

  return String(value)
}
</script>

<template>
  <div class="page-shell" data-testid="storage-sqlite-detail-page">
    <div class="page-header">
      <div>
        <div class="eyebrow">STORAGE DETAIL</div>
        <h1 class="page-title">{{ displayName }}</h1>
        <p class="page-description">
          Browse row-by-row SQLite data for this table. Each column is rendered explicitly so you can inspect the full
          payload without leaving the embedded dashboard.
        </p>
      </div>
      <div class="header-actions">
        <v-chip color="success" variant="tonal" prepend-icon="mdi-table-large">
          {{ rowCountLabel }}
        </v-chip>
        <v-chip color="secondary" variant="tonal" prepend-icon="mdi-table-column">
          {{ columnCountLabel }}
        </v-chip>
        <v-btn variant="outlined" prepend-icon="mdi-arrow-left" :to="{ name: 'storage-sqlite' }">BACK TO TABLES</v-btn>
      </div>
    </div>

    <v-alert
      v-if="error"
      type="warning"
      variant="tonal"
      border="start"
      title="Failed to fetch SQLite table detail"
      :text="error"
    />

    <v-card v-if="detail" rounded="xl" variant="outlined">
      <v-card-text class="meta-grid">
        <div class="meta-item">
          <div class="meta-label">Table Name</div>
          <div class="meta-value">{{ detail.name }}</div>
        </div>
        <div class="meta-item">
          <div class="meta-label">Table Type</div>
          <div class="meta-value">{{ detail.type || '-' }}</div>
        </div>
        <div class="meta-item">
          <div class="meta-label">Primary Keys</div>
          <div class="chip-wrap">
            <v-chip
              v-for="primaryKey in detail.primaryKeys"
              :key="`${detail.name}-${primaryKey}`"
              size="x-small"
              variant="outlined"
            >
              {{ primaryKey }}
            </v-chip>
            <span v-if="!detail.primaryKeys.length">-</span>
          </div>
        </div>
      </v-card-text>
    </v-card>

    <div class="table-shell">
      <v-table class="detail-table" density="comfortable">
        <thead>
          <tr>
            <th>#</th>
            <th v-for="column in detail?.columns || []" :key="column">{{ column }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading">
            <td :colspan="(detail?.columns.length || 0) + 1">
              <v-skeleton-loader type="table-row-divider@6" />
            </td>
          </tr>
          <tr v-for="(row, rowIndex) in detail?.rows || []" :key="`${currentPage}-${rowIndex}`">
            <td class="index-cell">{{ (currentPage - 1) * (detail?.perPage || 20) + rowIndex + 1 }}</td>
            <td v-for="column in detail?.columns || []" :key="`${rowIndex}-${column}`" class="value-cell">
              {{ formatCellValue(row[column]) }}
            </td>
          </tr>
          <tr v-if="!loading && detail && !detail.rows.length">
            <td :colspan="detail.columns.length + 1" class="empty-state">This table is currently empty.</td>
          </tr>
        </tbody>
      </v-table>
    </div>

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
  word-break: break-word;
}

.page-description {
  max-width: 56rem;
  margin: 0.85rem 0 0;
  color: #54667a;
  line-height: 1.7;
}

.header-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  justify-content: flex-end;
}

.meta-grid {
  display: grid;
  gap: 1rem;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
}

.meta-item {
  display: grid;
  gap: 0.4rem;
}

.meta-label {
  color: #607286;
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.meta-value {
  color: #223748;
  word-break: break-word;
}

.chip-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.table-shell {
  overflow-x: auto;
  border: 1px solid rgba(22, 50, 79, 0.08);
  border-radius: 1.5rem;
  background: #fff;
}

.detail-table :deep(th) {
  color: #5c6d7d;
  font-weight: 700;
  white-space: nowrap;
}

.detail-table :deep(td) {
  vertical-align: top;
}

.index-cell {
  color: #607286;
  font-weight: 700;
  white-space: nowrap;
}

.value-cell {
  min-width: 150px;
  color: #33495c;
  word-break: break-word;
  white-space: pre-wrap;
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

  .header-actions,
  .page-actions {
    width: 100%;
  }
}
</style>

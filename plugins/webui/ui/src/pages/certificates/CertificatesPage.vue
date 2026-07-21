<script setup lang="ts">
import { computed } from 'vue'

import { useGenericResourceCollection } from '../../composables/useGenericResourceCollection'
import { getResourceConfig } from '../../resource-config'

const config = getResourceConfig('certificates')

if (!config) {
  throw new Error('Missing resource config for certificates')
}

const { canGoNext, canGoPrevious, currentPage, direction, error, items, loading, pageCount, search, sortBy, status } =
  useGenericResourceCollection(config)

const statusOptions = [
  { title: 'All statuses', value: '' },
  { title: 'Enabled', value: 'enabled' },
  { title: 'Warning', value: 'warning' },
  { title: 'Expired', value: 'expired' },
]

const sortOptions = [
  { title: 'Common Name', value: 'name' },
  { title: 'Status', value: 'status' },
  { title: 'Resolver', value: 'resolver' },
  { title: 'Expires', value: 'notAfter' },
  { title: 'Issuer', value: 'issuerCN' },
]

const statusColor = computed(() => (value: string) => {
  switch (value) {
    case 'enabled':
      return 'success'
    case 'warning':
      return 'warning'
    case 'expired':
      return 'error'
    default:
      return 'secondary'
  }
})

function toggleDirection() {
  direction.value = direction.value === 'asc' ? 'desc' : 'asc'
}

function daysLeft(notAfter: unknown) {
  if (typeof notAfter !== 'string') {
    return null
  }

  return Math.floor((new Date(notAfter).getTime() - Date.now()) / (1000 * 60 * 60 * 24))
}

function expiryLabel(notAfter: unknown) {
  const days = daysLeft(notAfter)
  if (days === null) {
    return '-'
  }
  if (days < 0) {
    return 'EXPIRED'
  }
  return `${days} days`
}

function formatDate(value: unknown) {
  if (typeof value !== 'string') {
    return '-'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleDateString('zh-CN')
}
</script>

<template>
  <div class="page-shell" data-testid="certificates-page">
    <div class="page-header">
      <div>
        <div class="eyebrow">Certificates</div>
        <h1 class="page-title">Certificates</h1>
        <p class="page-description">
          Browse certificate inventory in the Vue UI, including status, SAN coverage, issuing authority,
          expiry date, and quick navigation into the dedicated certificate detail page.
        </p>
      </div>
      <v-chip color="success" variant="tonal" prepend-icon="mdi-certificate-outline">
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
              label="Search certificate name"
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
      title="Failed to fetch certificates"
      :text="error"
    />

    <v-table class="certificate-table" density="comfortable">
      <thead>
        <tr>
          <th>Status</th>
          <th>Common Name</th>
          <th>SANs</th>
          <th>Issuer</th>
          <th>Valid Until</th>
          <th>Expiry</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="loading">
          <td colspan="6">
            <v-skeleton-loader type="table-row-divider@6" />
          </td>
        </tr>
        <tr v-for="certificate in items" :key="String(certificate.name)">
          <td>
            <v-chip :color="statusColor(String(certificate.status || ''))" size="small" variant="tonal">
              {{ certificate.status || '-' }}
            </v-chip>
          </td>
          <td>
            <router-link class="name-link" :to="`/certificates/${encodeURIComponent(String(certificate.name || ''))}`">
              {{ certificate.commonName || '-' }}
            </router-link>
          </td>
          <td class="sans-cell">
            {{ Array.isArray(certificate.sans) && certificate.sans.length ? certificate.sans.join(', ') : '-' }}
          </td>
          <td>{{ certificate.issuerOrg || certificate.issuerCN || 'Unknown' }}</td>
          <td>{{ formatDate(certificate.notAfter) }}</td>
          <td>{{ expiryLabel(certificate.notAfter) }}</td>
        </tr>
        <tr v-if="!loading && !items.length">
          <td colspan="6" class="empty-state">No data available.</td>
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
.certificate-table :deep(th) { color: #5c6d7d; font-weight: 700; }
.certificate-table :deep(td) { vertical-align: top; }
.sans-cell { max-width: 24rem; color: #33495c; line-height: 1.6; word-break: break-word; }
.name-link { color: #1f8aa8; font-weight: 600; text-decoration: none; }
.name-link:hover { text-decoration: underline; }
.empty-state { color: #66788a; text-align: center; }
.pagination-row { display: flex; justify-content: space-between; gap: 1rem; align-items: center; }
.page-meta { color: #66788a; }
.page-actions { display: flex; gap: 0.75rem; }
@media (max-width: 960px) { .page-header, .pagination-row { flex-direction: column; align-items: stretch; } .page-actions { width: 100%; } }
</style>

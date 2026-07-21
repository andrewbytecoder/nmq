<script setup lang="ts">
import { computed } from 'vue'

import ResourceValueView from '../../components/ResourceValueView.vue'
import { useGenericResourceDetail } from '../../composables/useGenericResourceDetail'
import { getResourceConfig } from '../../resource-config'

type ServerEntry = {
  url?: string
  address?: string
  weight?: number
}

const config = getResourceConfig('tcp-services')

if (!config) {
  throw new Error('Missing resource config for tcp-services')
}

const { displayName, error, item, loading } = useGenericResourceDetail(config)

const summaryEntries = computed(() => {
  const source = item.value || {}
  return [
    { label: 'Name', value: source.name || displayName.value },
    { label: 'Status', value: source.status },
    { label: 'Provider', value: source.provider },
    { label: 'Type', value: source.type },
  ].filter((entry) => entry.value)
})

const loadBalancer = computed(() => (item.value?.loadBalancer as Record<string, unknown> | undefined) || undefined)
const servers = computed(() => ((loadBalancer.value?.servers as ServerEntry[]) || []))
const healthCheck = computed(() => (loadBalancer.value?.healthCheck as Record<string, unknown> | undefined) || undefined)
const weightedServices = computed(() => (((item.value?.weighted as { services?: Array<{ name: string; weight?: number }> })?.services) || []))
const usedBy = computed(() => (((item.value?.usedBy as string[]) || []).filter(Boolean)))
const serverStatus = computed(() => ((item.value?.serverStatus as Record<string, string>) || {}))

const rawPayloadEntries = computed(() => {
  if (!item.value) {
    return []
  }

  const hiddenKeys = new Set(['name', 'status', 'provider', 'type', 'loadBalancer', 'weighted', 'usedBy', 'serverStatus'])
  return Object.entries(item.value).filter(([key]) => !hiddenKeys.has(key))
})

function serverAddress(server: ServerEntry) {
  return server.address || server.url || '-'
}
</script>

<template>
  <div class="detail-shell" data-testid="tcp-service-detail-page">
    <div class="detail-header">
      <div>
        <div class="eyebrow">TCP</div>
        <h1 class="detail-title">{{ displayName }}</h1>
        <p class="detail-description">
          Inspect the selected TCP service, including backend addresses, connection-level health checks,
          termination delay, weighted targets, and routers that depend on the service.
        </p>
      </div>
      <v-chip color="primary" variant="tonal" prepend-icon="mdi-server-network-outline">
        TCP Service
      </v-chip>
    </div>

    <v-alert
      v-if="error"
      type="warning"
      variant="tonal"
      border="start"
      title="Failed to fetch TCP service detail"
      :text="error"
    />

    <template v-if="loading">
      <v-row dense>
        <v-col v-for="index in 3" :key="`tcp-service-detail-skeleton-${index}`" cols="12" md="4">
          <v-skeleton-loader type="card" />
        </v-col>
      </v-row>
    </template>

    <template v-else-if="item">
      <v-row dense>
        <v-col cols="12" md="4">
          <v-card rounded="xl" variant="elevated" class="fill-height">
            <v-card-title>Summary</v-card-title>
            <v-card-text class="summary-grid">
              <div v-for="entry in summaryEntries" :key="entry.label" class="summary-item">
                <div class="summary-label">{{ entry.label }}</div>
                <div class="summary-value">{{ entry.value }}</div>
              </div>
              <div v-if="loadBalancer?.terminationDelay !== undefined" class="summary-item">
                <div class="summary-label">Termination delay</div>
                <div class="summary-value">{{ loadBalancer.terminationDelay }} ms</div>
              </div>
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="8">
          <v-card rounded="xl" variant="outlined" class="fill-height">
            <v-card-title>Backend Servers</v-card-title>
            <v-card-text>
              <template v-if="servers.length">
                <v-table density="comfortable">
                  <thead>
                    <tr>
                      <th>Status</th>
                      <th>Address</th>
                      <th>Weight</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="server in servers" :key="serverAddress(server)">
                      <td>{{ serverStatus[serverAddress(server)] || 'DOWN' }}</td>
                      <td>{{ serverAddress(server) }}</td>
                      <td>{{ server.weight ?? 1 }}</td>
                    </tr>
                  </tbody>
                </v-table>
              </template>
              <div v-else class="empty-copy">No backend servers are defined for this TCP service.</div>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <v-row dense>
        <v-col cols="12" md="6">
          <v-card rounded="xl" variant="outlined" class="fill-height">
            <v-card-title>Health Check</v-card-title>
            <v-card-text>
              <template v-if="healthCheck">
                <div class="payload-grid">
                  <ResourceValueView v-for="[key, value] in Object.entries(healthCheck)" :key="key" :label="key" :value="value" />
                </div>
              </template>
              <div v-else class="empty-copy">No active health check is configured.</div>
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="6">
          <v-card rounded="xl" variant="outlined" class="fill-height">
            <v-card-title>Used By Routers</v-card-title>
            <v-card-text>
              <template v-if="usedBy.length">
                <div class="chip-wrap">
                  <router-link v-for="routerName in usedBy" :key="routerName" class="chip-link" :to="`/tcp/routers/${encodeURIComponent(routerName)}`">
                    <v-chip size="small" variant="outlined">{{ routerName }}</v-chip>
                  </router-link>
                </div>
              </template>
              <div v-else class="empty-copy">No routers reference this TCP service.</div>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <v-card rounded="xl" variant="elevated">
        <v-card-title>Weighted Targets</v-card-title>
        <v-card-text>
          <template v-if="weightedServices.length">
            <v-table density="comfortable">
              <thead>
                <tr>
                  <th>Service</th>
                  <th>Weight</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="target in weightedServices" :key="target.name">
                  <td>{{ target.name }}</td>
                  <td>{{ target.weight ?? '-' }}</td>
                </tr>
              </tbody>
            </v-table>
          </template>
          <div v-else class="empty-copy">This service does not use weighted child targets.</div>
        </v-card-text>
      </v-card>

      <v-card rounded="xl" variant="outlined">
        <v-card-title>Additional Payload</v-card-title>
        <v-card-text>
          <template v-if="rawPayloadEntries.length">
            <div class="payload-grid">
              <ResourceValueView v-for="[key, value] in rawPayloadEntries" :key="key" :label="key" :value="value" />
            </div>
          </template>
          <div v-else class="empty-copy">No additional payload fields to display.</div>
        </v-card-text>
      </v-card>
    </template>
  </div>
</template>

<style scoped>
.detail-shell { display: grid; gap: 1.25rem; }
.detail-header { display: flex; justify-content: space-between; gap: 1rem; align-items: flex-start; }
.eyebrow { margin-bottom: 0.5rem; color: #1f8aa8; font-size: 0.8rem; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase; }
.detail-title { margin: 0; color: #16324f; font-size: clamp(1.9rem, 3vw, 2.8rem); word-break: break-word; }
.detail-description { max-width: 56rem; margin: 0.85rem 0 0; color: #54667a; line-height: 1.7; }
.summary-grid, .payload-grid { display: grid; gap: 1rem; }
.summary-item { display: grid; gap: 0.4rem; }
.summary-label { color: #607286; font-size: 0.8rem; font-weight: 700; letter-spacing: 0.04em; text-transform: uppercase; }
.summary-value { color: #223748; line-height: 1.7; word-break: break-word; }
.chip-wrap { display: flex; flex-wrap: wrap; gap: 0.45rem; }
.chip-link { text-decoration: none; }
.empty-copy { color: #66788a; }
@media (max-width: 960px) { .detail-header { flex-direction: column; align-items: stretch; } }
</style>

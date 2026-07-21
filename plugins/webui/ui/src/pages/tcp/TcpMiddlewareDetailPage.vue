<script setup lang="ts">
import { computed } from 'vue'

import ResourceValueView from '../../components/ResourceValueView.vue'
import { useGenericResourceDetail } from '../../composables/useGenericResourceDetail'
import { getResourceConfig } from '../../resource-config'

const config = getResourceConfig('tcp-middlewares')

if (!config) {
  throw new Error('Missing resource config for tcp-middlewares')
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

const usedBy = computed(() => (((item.value?.usedBy as string[]) || []).filter(Boolean)))

const configEntries = computed(() => {
  if (!item.value) {
    return []
  }

  const hiddenKeys = new Set(['name', 'status', 'provider', 'type', 'usedBy', 'error', 'routers'])
  return Object.entries(item.value).filter(([key]) => !hiddenKeys.has(key))
})
</script>

<template>
  <div class="detail-shell" data-testid="tcp-middleware-detail-page">
    <div class="detail-header">
      <div>
        <div class="eyebrow">TCP</div>
        <h1 class="detail-title">{{ displayName }}</h1>
        <p class="detail-description">
          Inspect the selected TCP middleware, including the routers that depend on it and the middleware's
          protocol-specific configuration payload.
        </p>
      </div>
      <v-chip color="primary" variant="tonal" prepend-icon="mdi-tune-variant">
        TCP Middleware
      </v-chip>
    </div>

    <v-alert
      v-if="error"
      type="warning"
      variant="tonal"
      border="start"
      title="Failed to fetch TCP middleware detail"
      :text="error"
    />

    <template v-if="loading">
      <v-row dense>
        <v-col v-for="index in 3" :key="`tcp-middleware-detail-skeleton-${index}`" cols="12" md="4">
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
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="8">
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
              <div v-else class="empty-copy">No routers reference this middleware.</div>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <v-card rounded="xl" variant="elevated">
        <v-card-title>Middleware Configuration</v-card-title>
        <v-card-text>
          <template v-if="configEntries.length">
            <div class="payload-grid">
              <ResourceValueView v-for="[key, value] in configEntries" :key="key" :label="key" :value="value" />
            </div>
          </template>
          <div v-else class="empty-copy">No middleware-specific payload fields to display.</div>
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

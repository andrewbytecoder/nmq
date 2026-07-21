<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import ResourceValueView from '../../components/ResourceValueView.vue'
import { useGenericResourceDetail } from '../../composables/useGenericResourceDetail'
import { fetchJSON } from '../../lib/api'
import { getResourceConfig } from '../../resource-config'

type EntrypointInfo = {
  name: string
  address: string
}

const config = getResourceConfig('tcp-routers')

if (!config) {
  throw new Error('Missing resource config for tcp-routers')
}

const route = useRoute()
const { displayName, error, item, loading } = useGenericResourceDetail(config)

const entrypoints = ref<EntrypointInfo[]>([])

onMounted(async () => {
  try {
    entrypoints.value = await fetchJSON<EntrypointInfo[]>('/entrypoints')
  } catch {
    entrypoints.value = []
  }
})

const summaryEntries = computed(() => {
  const source = item.value || {}
  return [
    { label: 'Name', value: source.name || displayName.value },
    { label: 'Status', value: source.status },
    { label: 'Provider', value: source.provider },
    { label: 'Service', value: source.service },
    { label: 'Priority', value: source.priorityStr || source.priority },
  ].filter((entry) => entry.value)
})

const linkedEntrypoints = computed(() => {
  const source = item.value || {}
  const names = ((source.using as string[]) || (source.entryPoints as string[]) || []).filter(Boolean)
  return names.map((name) => entrypoints.value.find((entry) => entry.name === name) || { name, address: '-' })
})

const middlewareNames = computed(() => (((item.value?.middlewares as string[]) || []).filter(Boolean)))

const tlsOptions = computed(() => {
  const tls = item.value?.tls as { options?: string; certResolver?: string } | undefined
  if (!tls) {
    return []
  }

  return [
    { label: 'Options', value: tls.options || '-' },
    { label: 'Cert resolver', value: tls.certResolver || '-' },
  ]
})

const rawPayloadEntries = computed(() => {
  if (!item.value) {
    return []
  }

  const hiddenKeys = new Set([
    'name',
    'status',
    'provider',
    'service',
    'priority',
    'priorityStr',
    'using',
    'entryPoints',
    'middlewares',
    'tls',
    'rule',
    'error',
  ])
  return Object.entries(item.value).filter(([key]) => !hiddenKeys.has(key))
})

const errorMessages = computed(() => (Array.isArray(item.value?.error) ? (item.value?.error as string[]) : []))

const detailRouteBase = computed(() => `/tcp/routers/${encodeURIComponent(String(route.params.name || ''))}`)
</script>

<template>
  <div class="detail-shell" data-testid="tcp-router-detail-page">
    <div class="detail-header">
      <div>
        <div class="eyebrow">TCP</div>
        <h1 class="detail-title">{{ displayName }}</h1>
        <p class="detail-description">
          Inspect the selected TCP router, including SNI rule matching, linked service, entrypoints,
          middlewares, TLS settings, and any runtime routing errors.
        </p>
      </div>
      <v-chip color="primary" variant="tonal" prepend-icon="mdi-lan-connect">
        TCP Router
      </v-chip>
    </div>

    <v-alert
      v-if="error"
      type="warning"
      variant="tonal"
      border="start"
      title="Failed to fetch TCP router detail"
      :text="error"
    />

    <template v-if="loading">
      <v-row dense>
        <v-col v-for="index in 3" :key="`tcp-router-detail-skeleton-${index}`" cols="12" md="4">
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
            <v-card-title>SNI Rule</v-card-title>
            <v-card-text class="rule-block">{{ item.rule || '-' }}</v-card-text>
            <v-divider />
            <v-card-text class="link-grid">
              <div>
                <div class="summary-label">Linked service</div>
                <router-link v-if="item.service" class="name-link" :to="`/tcp/services/${encodeURIComponent(String(item.service))}`">
                  {{ item.service }}
                </router-link>
                <div v-else class="summary-value">-</div>
              </div>
              <div>
                <div class="summary-label">Detail route</div>
                <div class="summary-value">{{ detailRouteBase }}</div>
              </div>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <v-row dense>
        <v-col cols="12" md="6">
          <v-card rounded="xl" variant="outlined" class="fill-height">
            <v-card-title>Entrypoints</v-card-title>
            <v-card-text>
              <template v-if="linkedEntrypoints.length">
                <div class="linked-list">
                  <div v-for="entry in linkedEntrypoints" :key="entry.name" class="linked-item">
                    <div class="summary-label">{{ entry.name }}</div>
                    <div class="summary-value">{{ entry.address }}</div>
                  </div>
                </div>
              </template>
              <div v-else class="empty-copy">No entrypoints linked to this router.</div>
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="6">
          <v-card rounded="xl" variant="outlined" class="fill-height">
            <v-card-title>Middlewares</v-card-title>
            <v-card-text>
              <template v-if="middlewareNames.length">
                <div class="chip-wrap">
                  <router-link
                    v-for="middlewareName in middlewareNames"
                    :key="middlewareName"
                    class="chip-link"
                    :to="`/tcp/middlewares/${encodeURIComponent(middlewareName)}`"
                  >
                    <v-chip size="small" variant="outlined">{{ middlewareName }}</v-chip>
                  </router-link>
                </div>
              </template>
              <div v-else class="empty-copy">No middlewares linked to this router.</div>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <v-card rounded="xl" variant="elevated">
        <v-card-title>TLS</v-card-title>
        <v-card-text>
          <template v-if="item.tls">
            <div class="tls-grid">
              <div v-for="entry in tlsOptions" :key="entry.label" class="summary-item">
                <div class="summary-label">{{ entry.label }}</div>
                <div class="summary-value">{{ entry.value }}</div>
              </div>
            </div>
          </template>
          <div v-else class="empty-copy">TLS is not configured for this router.</div>
        </v-card-text>
      </v-card>

      <v-card v-if="errorMessages.length" rounded="xl" variant="outlined">
        <v-card-title>Errors</v-card-title>
        <v-card-text>
          <v-alert
            v-for="message in errorMessages"
            :key="message"
            type="error"
            variant="tonal"
            class="mb-3"
            :text="message"
          />
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
.summary-grid, .payload-grid, .tls-grid, .linked-list, .link-grid { display: grid; gap: 1rem; }
.summary-item { display: grid; gap: 0.4rem; }
.summary-label { color: #607286; font-size: 0.8rem; font-weight: 700; letter-spacing: 0.04em; text-transform: uppercase; }
.summary-value, .rule-block { color: #223748; line-height: 1.7; word-break: break-word; }
.rule-block { white-space: pre-wrap; }
.chip-wrap { display: flex; flex-wrap: wrap; gap: 0.45rem; }
.chip-link { text-decoration: none; }
.linked-item { display: grid; gap: 0.25rem; padding: 0.9rem 1rem; border: 1px solid rgba(34,55,72,0.08); border-radius: 1rem; }
.name-link { color: #1f8aa8; font-weight: 600; text-decoration: none; }
.name-link:hover { text-decoration: underline; }
.empty-copy { color: #66788a; }
@media (max-width: 960px) { .detail-header { flex-direction: column; align-items: stretch; } }
</style>

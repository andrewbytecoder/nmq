<script setup lang="ts">
import { computed } from 'vue'

import ResourceValueView from '../components/ResourceValueView.vue'
import { useGenericResourceDetail } from '../composables/useGenericResourceDetail'
import { getResourceConfig } from '../resource-config'

const props = defineProps<{
  resourceKey: string
}>()

const config = getResourceConfig(props.resourceKey)

if (!config) {
  throw new Error(`Unknown resource config: ${props.resourceKey}`)
}

const { displayName, error, item, loading } = useGenericResourceDetail(config)

const summaryEntries = computed(() => {
  const source = item.value || {}
  return [
    { label: 'Name', value: source.name || displayName.value },
    { label: 'Status', value: source.status },
    { label: 'Provider', value: source.provider },
    { label: 'Type', value: source.type },
    { label: 'Service', value: source.service },
    { label: 'Rule', value: source.rule },
    { label: 'Resolver', value: source.resolver },
    { label: 'Expires', value: source.notAfter },
  ].filter((entry) => entry.value)
})

const relationshipEntries = computed(() => {
  const source = item.value || {}
  return [
    { label: 'Entrypoints', value: source.entryPoints || source.using },
    { label: 'Used By', value: source.usedBy },
    { label: 'Middlewares', value: source.middlewares },
    { label: 'Routers', value: source.routers },
    { label: 'SANs', value: source.sans },
  ].filter((entry) => Array.isArray(entry.value) && entry.value.length)
})

const messageEntries = computed(() => {
  const source = item.value || {}
  const items = []
  if (Array.isArray(source.error) && source.error.length) {
    items.push({ label: 'Errors', value: source.error })
  }
  if (source.message) {
    items.push({ label: 'Message', value: source.message })
  }
  return items
})

const payloadEntries = computed(() => {
  if (!item.value) {
    return []
  }

  const hiddenKeys = new Set([
    'name',
    'status',
    'provider',
    'type',
    'service',
    'rule',
    'resolver',
    'notAfter',
    'entryPoints',
    'using',
    'usedBy',
    'middlewares',
    'routers',
    'sans',
    'error',
    'message',
  ])

  return Object.entries(item.value).filter(([key]) => !hiddenKeys.has(key))
})

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
</script>

<template>
  <div class="detail-shell" :data-testid="`${config.key}-detail-page`">
    <div class="detail-header">
      <div>
        <div class="eyebrow">{{ config.section }}</div>
        <h1 class="detail-title">{{ displayName }}</h1>
        <p class="detail-description">{{ config.detailDescription }}</p>
      </div>
      <v-chip
        v-if="item?.status"
        :color="statusColor(String(item.status))"
        variant="tonal"
        prepend-icon="mdi-information-outline"
      >
        {{ item.status }}
      </v-chip>
    </div>

    <v-alert
      v-if="error"
      type="warning"
      variant="tonal"
      border="start"
      :title="`Failed to fetch ${config.title} detail`"
      :text="error"
    />

    <template v-if="loading">
      <v-row dense>
        <v-col v-for="index in 3" :key="`detail-skeleton-${index}`" cols="12" md="4">
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

        <v-col cols="12" md="4">
          <v-card rounded="xl" variant="outlined" class="fill-height">
            <v-card-title>Relationships</v-card-title>
            <v-card-text>
              <template v-if="relationshipEntries.length">
                <div v-for="entry in relationshipEntries" :key="entry.label" class="relationship-block">
                  <div class="summary-label">{{ entry.label }}</div>
                  <div class="chip-wrap">
                    <v-chip v-for="value in entry.value as unknown[]" :key="String(value)" size="small" variant="outlined">
                      {{ String(value) }}
                    </v-chip>
                  </div>
                </div>
              </template>
              <div v-else class="empty-copy">No linked relationships to display.</div>
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="4">
          <v-card rounded="xl" variant="outlined" class="fill-height">
            <v-card-title>Messages</v-card-title>
            <v-card-text>
              <template v-if="messageEntries.length">
                <div v-for="entry in messageEntries" :key="entry.label" class="relationship-block">
                  <ResourceValueView :label="entry.label" :value="entry.value" />
                </div>
              </template>
              <div v-else class="empty-copy">No warnings or messages reported for this resource.</div>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <v-card rounded="xl" variant="elevated">
        <v-card-title>Configuration Payload</v-card-title>
        <v-card-text>
          <template v-if="payloadEntries.length">
            <div class="payload-grid">
              <ResourceValueView v-for="[key, value] in payloadEntries" :key="key" :label="key" :value="value" />
            </div>
          </template>
          <div v-else class="empty-copy">No additional payload fields to display.</div>
        </v-card-text>
      </v-card>
    </template>
  </div>
</template>

<style scoped>
.detail-shell {
  display: grid;
  gap: 1.25rem;
}

.detail-header {
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

.detail-title {
  margin: 0;
  color: #16324f;
  font-size: clamp(1.9rem, 3vw, 2.8rem);
  word-break: break-word;
}

.detail-description {
  max-width: 56rem;
  margin: 0.85rem 0 0;
  color: #54667a;
  line-height: 1.7;
}

.summary-grid,
.payload-grid {
  display: grid;
  gap: 1rem;
}

.summary-item,
.relationship-block {
  display: grid;
  gap: 0.5rem;
}

.summary-label {
  color: #607286;
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.summary-value {
  color: #223748;
  line-height: 1.7;
  word-break: break-word;
}

.chip-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.empty-copy {
  color: #66788a;
}

@media (max-width: 960px) {
  .detail-header {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>

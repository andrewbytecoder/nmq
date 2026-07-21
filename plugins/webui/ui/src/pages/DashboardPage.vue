<script setup lang="ts">
import { computed } from 'vue'

import { useDashboardOverview } from '../composables/useDashboardOverview'

const defaultResourceOrder = ['routers', 'services', 'middlewares'] as const

const providerIcons: Record<string, string> = {
  docker: 'mdi-docker',
  kubernetes: 'mdi-kubernetes',
  file: 'mdi-file-document-outline',
  internal: 'mdi-shield-outline',
  ecs: 'mdi-aws',
  consul: 'mdi-database-outline',
  redis: 'mdi-database',
  nomad: 'mdi-hexagon-multiple-outline',
}

const { loading, error, entrypoints, overview, features, resourceGroups, reload } = useDashboardOverview()

const providerList = computed(() => overview.value?.providers ?? [])

function titleCase(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1)
}

function providerIcon(name: string) {
  return providerIcons[name.toLowerCase()] ?? 'mdi-cloud-outline'
}
</script>

<template>
  <div class="dashboard-shell">
    <section class="hero-strip">
      <div>
        <div class="eyebrow">Overview</div>
        <h1 class="hero-title">Traefik dashboard, now running on Vue 3 and Vuetify.</h1>
        <p class="hero-copy">
          This page ports the top-level dashboard experience first, so the embedded frontend can already
          show live overview data while deeper resource pages migrate incrementally from the legacy React
          implementation.
        </p>
      </div>
      <div class="hero-actions">
        <v-btn color="primary" prepend-icon="mdi-refresh" @click="reload">Refresh</v-btn>
        <v-chip color="secondary" variant="tonal">Static build path preserved</v-chip>
      </div>
    </section>

    <v-alert
      v-if="error"
      type="warning"
      variant="tonal"
      border="start"
      title="Dashboard data is unavailable"
      :text="error"
    />

    <template v-if="loading">
      <v-row dense>
        <v-col v-for="index in 4" :key="`skeleton-entry-${index}`" cols="12" md="3">
          <v-skeleton-loader type="article" />
        </v-col>
      </v-row>
      <v-row dense class="mt-2">
        <v-col v-for="index in 6" :key="`skeleton-card-${index}`" cols="12" md="4">
          <v-skeleton-loader type="card" />
        </v-col>
      </v-row>
    </template>

    <template v-else>
      <section class="dashboard-section">
        <div class="section-heading">
          <h2>Entrypoints</h2>
          <span>{{ entrypoints.length }} configured</span>
        </div>
        <v-row dense>
          <v-col v-for="entrypoint in entrypoints" :key="entrypoint.name" cols="12" md="4" lg="3">
            <v-card rounded="xl" variant="elevated" class="fill-height">
              <v-card-item>
                <v-card-title>{{ entrypoint.name }}</v-card-title>
                <v-card-subtitle>{{ entrypoint.address }}</v-card-subtitle>
              </v-card-item>
            </v-card>
          </v-col>
          <v-col v-if="!entrypoints.length" cols="12">
            <v-alert type="info" variant="tonal" text="No entrypoints are currently exposed by the API." />
          </v-col>
        </v-row>
      </section>

      <section v-for="group in resourceGroups" :key="group.key" class="dashboard-section">
        <div class="section-heading">
          <h2>{{ group.title }}</h2>
          <span>{{ group.routePrefix }}</span>
        </div>
        <v-row dense>
          <template v-if="group.hasData">
            <v-col
              v-for="resourceKey in group.key === 'udp' ? ['routers', 'services'] : defaultResourceOrder"
              :key="`${group.key}-${resourceKey}`"
              cols="12"
              md="4"
            >
              <v-card :to="`${group.routePrefix}/${resourceKey}`" rounded="xl" variant="elevated" class="stats-card">
                <v-card-item>
                  <template #prepend>
                    <v-avatar color="primary" variant="tonal">
                      <v-icon icon="mdi-chart-box-outline" />
                    </v-avatar>
                  </template>
                  <v-card-title>{{ titleCase(resourceKey) }}</v-card-title>
                  <v-card-subtitle>Open {{ group.title }} {{ resourceKey }} page</v-card-subtitle>
                </v-card-item>
                <v-card-text>
                  <div class="stats-grid">
                    <div class="stat-pill">
                      <span class="stat-label">Total</span>
                      <strong>{{ group.stats[resourceKey]?.total ?? 0 }}</strong>
                    </div>
                    <div class="stat-pill warning">
                      <span class="stat-label">Warnings</span>
                      <strong>{{ group.stats[resourceKey]?.warnings ?? 0 }}</strong>
                    </div>
                    <div class="stat-pill danger">
                      <span class="stat-label">Errors</span>
                      <strong>{{ group.stats[resourceKey]?.errors ?? 0 }}</strong>
                    </div>
                  </div>
                </v-card-text>
              </v-card>
            </v-col>
          </template>
          <v-col v-else cols="12">
            <v-alert type="info" variant="tonal" text="No related objects to show." />
          </v-col>
        </v-row>
      </section>

      <section class="dashboard-section">
        <div class="section-heading">
          <h2>Features</h2>
          <span>{{ features.length }} detected</span>
        </div>
        <v-row dense>
          <v-col v-for="feature in features" :key="feature.name" cols="12" md="4" lg="3">
            <v-card rounded="xl" variant="outlined" class="fill-height">
              <v-card-item>
                <template #prepend>
                  <v-avatar color="success" variant="tonal">
                    <v-icon icon="mdi-star-four-points-outline" />
                  </v-avatar>
                </template>
                <v-card-title>{{ titleCase(feature.name) }}</v-card-title>
                <v-card-subtitle>{{ String(feature.value) }}</v-card-subtitle>
              </v-card-item>
            </v-card>
          </v-col>
          <v-col v-if="!features.length" cols="12">
            <v-alert type="info" variant="tonal" text="No related objects to show." />
          </v-col>
        </v-row>
      </section>

      <section class="dashboard-section">
        <div class="section-heading">
          <h2>Providers</h2>
          <span>{{ providerList.length }} active</span>
        </div>
        <v-row dense>
          <v-col v-for="provider in providerList" :key="provider" cols="12" md="4" lg="3">
            <v-card rounded="xl" variant="elevated" class="fill-height provider-card">
              <v-icon :icon="providerIcon(provider)" size="40" color="primary" />
              <div class="provider-name">{{ provider }}</div>
            </v-card>
          </v-col>
          <v-col v-if="!providerList.length" cols="12">
            <v-alert type="info" variant="tonal" text="No related objects to show." />
          </v-col>
        </v-row>
      </section>
    </template>
  </div>
</template>

<style scoped>
.dashboard-shell {
  display: grid;
  gap: 1.5rem;
}

.hero-strip {
  display: flex;
  gap: 1.5rem;
  justify-content: space-between;
  padding: 1.5rem;
  border-radius: 1.5rem;
  background:
    radial-gradient(circle at top right, rgba(123, 211, 137, 0.25), transparent 26%),
    linear-gradient(135deg, rgba(31, 138, 168, 0.14) 0%, rgba(84, 180, 214, 0.08) 100%);
}

.eyebrow {
  margin-bottom: 0.6rem;
  color: #1f8aa8;
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.hero-title {
  margin: 0;
  max-width: 48rem;
  color: #16324f;
  font-size: clamp(2rem, 4vw, 3.2rem);
  line-height: 1.05;
}

.hero-copy {
  max-width: 52rem;
  margin: 1rem 0 0;
  color: #516274;
  line-height: 1.75;
}

.hero-actions {
  display: flex;
  gap: 0.75rem;
  align-items: flex-start;
  flex-wrap: wrap;
}

.dashboard-section {
  display: grid;
  gap: 1rem;
}

.section-heading {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: baseline;
}

.section-heading h2 {
  margin: 0;
  color: #16324f;
  font-size: 1.4rem;
}

.section-heading span {
  color: #708090;
  font-size: 0.95rem;
}

.stats-card {
  cursor: pointer;
}

.stats-grid {
  display: grid;
  gap: 0.75rem;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.stat-pill {
  display: grid;
  gap: 0.2rem;
  padding: 0.9rem;
  border-radius: 1rem;
  background: #eef5fb;
}

.stat-pill.warning {
  background: #fff6df;
}

.stat-pill.danger {
  background: #fdeeee;
}

.stat-label {
  color: #66788a;
  font-size: 0.78rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.provider-card {
  display: grid;
  place-items: center;
  gap: 0.85rem;
  min-height: 11rem;
  text-transform: capitalize;
}

.provider-name {
  color: #203244;
  font-size: 1rem;
  font-weight: 600;
}

@media (max-width: 960px) {
  .hero-strip {
    flex-direction: column;
  }

  .stats-grid {
    grid-template-columns: 1fr;
  }
}
</style>

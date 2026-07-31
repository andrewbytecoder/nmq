<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useDisplay, useTheme } from 'vuetify'

import { useHubDemoManifest } from './composables/useHubDemoManifest'
import { navSections } from './navigation'

const drawer = ref(true)
const rail = ref(false)
const route = useRoute()
const display = useDisplay()
const theme = useTheme()
const { navSection: hubDemoNavSection } = useHubDemoManifest()

const usesTemporaryDrawer = computed(() => display.mdAndDown.value)
const isRail = computed(() => rail.value && !usesTemporaryDrawer.value)
const drawerOffset = computed(() => {
  if (usesTemporaryDrawer.value || !drawer.value) {
    return 0
  }

  return isRail.value ? 72 : 296
})
const pageTitle = computed(() => (typeof route.meta.title === 'string' ? route.meta.title : 'Dashboard'))
const mergedNavSections = computed(() => [...navSections, hubDemoNavSection.value])

const isDark = computed(() => theme.global.name.value === 'dark')
const appShellStyle = computed(() => ({
  '--app-drawer-offset': `${drawerOffset.value}px`,
  '--app-bar-height': '64px',
}))

watch(
  usesTemporaryDrawer,
  (isTemporary) => {
    if (isTemporary) {
      rail.value = false
      return
    }

    drawer.value = true
  },
  { immediate: true },
)

function toggleTheme() {
  theme.global.name.value = isDark.value ? 'light' : 'dark'
}

function toggleDrawer() {
  if (usesTemporaryDrawer.value) {
    drawer.value = !drawer.value
    return
  }

  if (!drawer.value) {
    drawer.value = true
    rail.value = false
    return
  }

  rail.value = !rail.value
}
</script>

<template>
  <v-app :style="appShellStyle">
    <v-navigation-drawer
      v-model="drawer"
      :rail="isRail"
      :rail-width="72"
      :temporary="usesTemporaryDrawer"
      width="296"
      color="secondary"
    >
      <div class="drawer-header">
        <div class="brand-lockup">
          <div class="brand-mark">T</div>
          <div v-if="!isRail" class="brand-copy">
            <div class="brand-title">NCP</div>
            <div class="brand-subtitle">Not only Cloud Platform</div>
          </div>
        </div>
      </div>

      <div class="drawer-body">
        <template v-for="section in mergedNavSections" :key="section.key">
          <div v-if="section.label && !isRail" class="section-label">
            {{ section.label }}
          </div>

          <v-list
            nav
            density="comfortable"
            bg-color="transparent"
            :class="{ 'nav-list--rail': isRail }"
          >
            <v-list-item
              v-for="item in section.items"
              :key="item.path"
              :to="item.path"
              :prepend-icon="isRail ? undefined : item.icon"
              :title="isRail ? undefined : item.label"
              rounded="xl"
              color="primary"
              :class="{ 'nav-list-item--rail': isRail }"
            >
              <template v-if="isRail" #prepend>
                <v-icon :icon="item.icon" />
              </template>
              <template v-if="isRail" #append></template>
            </v-list-item>
          </v-list>
        </template>
      </div>
    </v-navigation-drawer>

    <v-app-bar color="surface" flat border>
      <v-app-bar-nav-icon @click="toggleDrawer" />
      <v-app-bar-title>{{ pageTitle }}</v-app-bar-title>

      <template #append>
        <v-btn
          icon
          variant="text"
          :href="'https://doc.traefik.io/traefik/'"
          target="_blank"
          rel="noreferrer"
        >
          <v-icon icon="mdi-book-open-page-variant-outline" />
        </v-btn>
        <v-btn
          icon
          variant="text"
          href="https://github.com/traefik/traefik/"
          target="_blank"
          rel="noreferrer"
        >
          <v-icon icon="mdi-github" />
        </v-btn>
        <v-btn icon variant="text" @click="toggleTheme">
          <v-icon :icon="isDark ? 'mdi-weather-sunny' : 'mdi-weather-night'" />
        </v-btn>
      </template>
    </v-app-bar>

    <v-main>
      <v-container class="page-container" fluid>
        <router-view />
      </v-container>
    </v-main>
  </v-app>
</template>

<style scoped>
.drawer-header {
  padding: 1.25rem 1rem 1rem;
}

.brand-lockup {
  display: flex;
  align-items: center;
  gap: 0.9rem;
}

.brand-mark {
  display: grid;
  place-items: center;
  width: 2.6rem;
  height: 2.6rem;
  border-radius: 0.95rem;
  background: linear-gradient(135deg, #7bd389 0%, #49c3d1 100%);
  color: #082032;
  font-weight: 900;
  font-size: 1.1rem;
}

.brand-title {
  color: #f8fbff;
  font-size: 1rem;
  font-weight: 700;
}

.brand-subtitle {
  color: rgba(248, 251, 255, 0.68);
  font-size: 0.78rem;
}

.drawer-body {
  padding: 0 0.75rem 1.25rem;
}

.drawer-body :deep(.nav-list--rail .v-list-item--nav) {
  padding-inline: 0;
}

.drawer-body :deep(.nav-list-item--rail) {
  min-height: 3rem;
}

.drawer-body :deep(.nav-list-item--rail .v-list-item__prepend) {
  align-items: center;
  justify-content: center;
  inline-size: 100%;
  margin-inline-end: 0;
}

.drawer-body :deep(.nav-list-item--rail .v-list-item__spacer) {
  display: none;
}

.section-label {
  padding: 1.1rem 0.85rem 0.4rem;
  color: rgba(248, 251, 255, 0.7);
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.page-container {
  padding: 1.5rem;
  max-width: 1600px;
}
</style>

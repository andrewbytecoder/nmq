import { computed, onMounted, ref } from 'vue'

import type { NavItem, NavSection } from '../navigation'
import { PUBLIC_KEY } from '../pages/hub-demo/constants'
import type { HubDemoManifest } from '../types/hub-demo'
import verifySignature from '../utils/workers/scriptVerification'

const ROUTES_MANIFEST_URL = 'https://traefik.github.io/hub-ui-demo-app/config/routes.json'

const defaultHubNavItems: NavItem[] = [
  {
    path: '/hub-dashboard/dashboard',
    label: 'Hub Dashboard',
    icon: 'mdi-view-dashboard-outline',
    description: 'Preview the API Management demo dashboard inside the new Vue shell.',
  },
]

const hubIconMap: Record<string, string> = {
  dashboard: 'mdi-view-dashboard-outline',
  gateway: 'mdi-lan-connect',
  api: 'mdi-api',
  portal: 'mdi-monitor-dashboard',
}

let manifestPromise: Promise<HubDemoManifest | null> | null = null
let manifestCache: HubDemoManifest | null | undefined

async function loadManifest(): Promise<HubDemoManifest | null> {
  if (manifestCache !== undefined) {
    return manifestCache
  }

  if (typeof Worker === 'undefined') {
    manifestCache = null
    return manifestCache
  }

  if (!manifestPromise) {
    manifestPromise = (async () => {
      try {
        const { verified, scriptContent } = await verifySignature(
          ROUTES_MANIFEST_URL,
          `${ROUTES_MANIFEST_URL}.sig`,
          PUBLIC_KEY,
        )

        if (!verified || !scriptContent) {
          manifestCache = null
          return manifestCache
        }

        const textDecoder = new TextDecoder()
        const jsonString = textDecoder.decode(scriptContent)
        manifestCache = JSON.parse(jsonString) as HubDemoManifest
        return manifestCache
      } catch (error) {
        console.error('Failed to load hub demo manifest:', error)
        manifestCache = null
        return manifestCache
      }
    })()
  }

  return manifestPromise
}

export function resetHubDemoManifestCache() {
  manifestCache = undefined
  manifestPromise = null
}

export function useHubDemoManifest() {
  const manifest = ref<HubDemoManifest | null>(null)
  const loading = ref(false)

  onMounted(async () => {
    loading.value = true
    manifest.value = await loadManifest()
    loading.value = false
  })

  const navItems = computed<NavItem[]>(() => {
    if (!manifest.value?.routes?.length) {
      return defaultHubNavItems
    }

    return manifest.value.routes.map((route) => ({
      path: `/hub-dashboard${route.path}`,
      label: route.label,
      icon: hubIconMap[route.icon] || 'mdi-cloud-outline',
      description: `Open the ${route.label} API Management demo view.`,
    }))
  })

  const navSection = computed<NavSection>(() => ({
    key: 'hub-demo',
    label: 'API Management Demo',
    items: navItems.value,
  }))

  return {
    loading,
    manifest,
    navItems,
    navSection,
  }
}

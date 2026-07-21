<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useTheme } from 'vuetify'


import verifySignature from '../../utils/workers/scriptVerification'

import { PUBLIC_KEY } from './constants'
import { getHubDemoBlobUrl, setHubDemoBlobUrl } from './hubDemoCache'

const SCRIPT_URL = 'https://assets.traefik.io/hub-ui-demo.js'

const route = useRoute()
const theme = useTheme()

const verificationInProgress = ref(false)
const signatureVerified = ref(false)
const scriptError = ref(false)
const scriptBlobUrl = ref<string | null>(getHubDemoBlobUrl())
let mountedScript: HTMLScriptElement | null = null

const usedPath = computed(() => {
  const value = route.params.pathMatch
  if (Array.isArray(value)) {
    return value.join('/')
  }

  if (typeof value === 'string' && value.length > 0) {
    return value
  }

  return 'dashboard'
})

const isDarkMode = computed(() => theme.global.name.value === 'dark')

onMounted(async () => {
  if (getHubDemoBlobUrl()) {
    scriptBlobUrl.value = getHubDemoBlobUrl()
    signatureVerified.value = true
    return
  }

  if (typeof Worker === 'undefined') {
    scriptError.value = true
    return
  }

  verificationInProgress.value = true

  try {
    const { verified, scriptContent } = await verifySignature(SCRIPT_URL, `${SCRIPT_URL}.sig`, PUBLIC_KEY)

    if (!verified || !scriptContent) {
      scriptError.value = true
      return
    }

    const blob = new Blob([scriptContent], { type: 'application/javascript' })
    const blobUrl = URL.createObjectURL(blob)
    setHubDemoBlobUrl(blobUrl)

    scriptBlobUrl.value = blobUrl
    signatureVerified.value = true
  } catch {
    scriptError.value = true
  } finally {
    verificationInProgress.value = false
  }
})

watch(
  scriptBlobUrl,
  (value) => {
    if (!value || mountedScript) {
      return
    }

    const script = document.createElement('script')
    script.type = 'module'
    script.src = value
    document.head.appendChild(script)
    mountedScript = script
  },
  { immediate: true },
)

onUnmounted(() => {
  if (mountedScript) {
    mountedScript.remove()
    mountedScript = null
  }
})
</script>

<template>
  <div class="hub-shell" data-testid="hub-demo-page">
    <section class="hub-header">
      <div>
        <div class="eyebrow">API Management Demo</div>
        <h1 class="hub-title">Traefik Hub demo inside the Vue shell.</h1>
        <p class="hub-description">
          This route keeps the signed remote demo flow from the legacy WebUI while moving the surrounding
          navigation and page ownership into the new Vue application.
        </p>
      </div>
      <v-chip color="primary" variant="tonal" prepend-icon="mdi-shield-check-outline">
        Signed remote content
      </v-chip>
    </section>

    <div v-if="verificationInProgress" class="loading-state">
      <v-progress-circular indeterminate color="primary" size="48" data-testid="loading" />
    </div>

    <v-alert
      v-else-if="scriptError"
      type="warning"
      variant="tonal"
      border="start"
      title="Oops! We couldn't load the demo content."
      text="You can still learn more about Traefik Hub API Management on the official website and in the documentation."
    />

    <div v-else class="hub-frame">
      <hub-ui-demo-app
        :key="usedPath"
        :path="usedPath"
        :theme="isDarkMode ? 'dark' : 'light'"
        baseurl="#/hub-dashboard"
        :containercss="
          JSON.stringify({
            maxWidth: '1334px',
            margin: '0 auto',
            marginTop: '24px',
          })
        "
      ></hub-ui-demo-app>
    </div>
  </div>
</template>

<style scoped>
.hub-shell {
  display: grid;
  gap: 1.5rem;
}

.hub-header {
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

.hub-title {
  margin: 0;
  color: #16324f;
  font-size: clamp(1.9rem, 3vw, 2.8rem);
}

.hub-description {
  max-width: 56rem;
  margin: 0.85rem 0 0;
  color: #54667a;
  line-height: 1.7;
}

.loading-state {
  display: grid;
  justify-items: center;
  padding: 3rem 0;
}

.hub-frame {
  display: block;
}

@media (max-width: 960px) {
  .hub-header {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>

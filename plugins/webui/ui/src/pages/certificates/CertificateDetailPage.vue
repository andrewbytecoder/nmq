<script setup lang="ts">
import { computed } from 'vue'

import ResourceValueView from '../../components/ResourceValueView.vue'
import { useGenericResourceDetail } from '../../composables/useGenericResourceDetail'
import { getResourceConfig } from '../../resource-config'

const config = getResourceConfig('certificates')

if (!config) {
  throw new Error('Missing resource config for certificates')
}

const { displayName, error, item, loading } = useGenericResourceDetail(config)

const certificate = computed(() => item.value || {})

const daysLeft = computed(() => {
  if (!certificate.value.notAfter || typeof certificate.value.notAfter !== 'string') {
    return null
  }

  return Math.floor((new Date(certificate.value.notAfter).getTime() - Date.now()) / (1000 * 60 * 60 * 24))
})

const issuedToEntries = computed(() => [
  { label: 'Common Name', value: certificate.value.commonName || displayName.value },
  { label: 'Organization', value: certificate.value.organization || '-' },
  { label: 'Country', value: certificate.value.country || '-' },
  { label: 'Status', value: certificate.value.status || '-' },
].filter((entry) => entry.value))

const issuedByEntries = computed(() => [
  { label: 'Common Name', value: certificate.value.issuerCN || '-' },
  { label: 'Organization', value: certificate.value.issuerOrg || '-' },
  { label: 'Country', value: certificate.value.issuerCountry || '-' },
  { label: 'Resolver', value: certificate.value.resolver || '-' },
])

const validityEntries = computed(() => [
  { label: 'Valid From', value: certificate.value.notBefore || '-' },
  { label: 'Valid Until', value: certificate.value.notAfter || '-' },
  { label: 'Days Left', value: daysLeft.value ?? '-' },
])

const technicalEntries = computed(() => [
  { label: 'Version', value: certificate.value.version || '-' },
  { label: 'Serial Number', value: certificate.value.serialNumber || '-' },
  { label: 'Key Type', value: certificate.value.keyType || '-' },
  { label: 'Key Size', value: certificate.value.keySize ? `${certificate.value.keySize} bits` : '-' },
  { label: 'Signature Algorithm', value: certificate.value.signatureAlgorithm || '-' },
])

const fingerprintEntries = computed(() => [
  { label: 'Certificate', value: certificate.value.certFingerprint || '-' },
  { label: 'Public Key', value: certificate.value.publicKeyFingerprint || '-' },
])

const sans = computed(() => ((certificate.value.sans as string[]) || []))
</script>

<template>
  <div class="detail-shell" data-testid="certificate-detail-page">
    <div class="detail-header">
      <div>
        <div class="eyebrow">Certificates</div>
        <h1 class="detail-title">{{ certificate.commonName || displayName }}</h1>
        <p class="detail-description">
          Inspect the selected certificate, including issued-to metadata, issuer information, validity,
          technical properties, SAN coverage, and SHA-256 fingerprints.
        </p>
      </div>
      <v-chip color="primary" variant="tonal" prepend-icon="mdi-certificate-outline">
        Certificate
      </v-chip>
    </div>

    <v-alert
      v-if="error"
      type="warning"
      variant="tonal"
      border="start"
      title="Failed to fetch certificate detail"
      :text="error"
    />

    <template v-if="loading">
      <v-row dense>
        <v-col v-for="index in 3" :key="`certificate-detail-skeleton-${index}`" cols="12" md="4">
          <v-skeleton-loader type="card" />
        </v-col>
      </v-row>
    </template>

    <template v-else-if="item">
      <v-row dense>
        <v-col cols="12" md="4">
          <v-card rounded="xl" variant="elevated" class="fill-height">
            <v-card-title>Issued To</v-card-title>
            <v-card-text class="summary-grid">
              <div v-for="entry in issuedToEntries" :key="entry.label" class="summary-item">
                <div class="summary-label">{{ entry.label }}</div>
                <div class="summary-value">{{ entry.value }}</div>
              </div>
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="4">
          <v-card rounded="xl" variant="outlined" class="fill-height">
            <v-card-title>Issued By</v-card-title>
            <v-card-text class="summary-grid">
              <div v-for="entry in issuedByEntries" :key="entry.label" class="summary-item">
                <div class="summary-label">{{ entry.label }}</div>
                <div class="summary-value">{{ entry.value }}</div>
              </div>
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="4">
          <v-card rounded="xl" variant="outlined" class="fill-height">
            <v-card-title>Validity</v-card-title>
            <v-card-text class="summary-grid">
              <div v-for="entry in validityEntries" :key="entry.label" class="summary-item">
                <div class="summary-label">{{ entry.label }}</div>
                <div class="summary-value">{{ entry.value }}</div>
              </div>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <v-row dense>
        <v-col cols="12" md="6">
          <v-card rounded="xl" variant="elevated" class="fill-height">
            <v-card-title>Subject Alternative Names</v-card-title>
            <v-card-text>
              <template v-if="sans.length">
                <div class="chip-wrap">
                  <v-chip v-for="san in sans" :key="san" size="small" variant="outlined">{{ san }}</v-chip>
                </div>
              </template>
              <div v-else class="empty-copy">No subject alternative names are present.</div>
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="6">
          <v-card rounded="xl" variant="outlined" class="fill-height">
            <v-card-title>Technical Details</v-card-title>
            <v-card-text class="summary-grid">
              <div v-for="entry in technicalEntries" :key="entry.label" class="summary-item">
                <div class="summary-label">{{ entry.label }}</div>
                <div class="summary-value">{{ entry.value }}</div>
              </div>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <v-card rounded="xl" variant="outlined">
        <v-card-title>SHA-256 Fingerprints</v-card-title>
        <v-card-text class="summary-grid">
          <div v-for="entry in fingerprintEntries" :key="entry.label" class="summary-item">
            <div class="summary-label">{{ entry.label }}</div>
            <div class="summary-value">{{ entry.value }}</div>
          </div>
        </v-card-text>
      </v-card>

      <v-card rounded="xl" variant="outlined">
        <v-card-title>Raw Certificate Payload</v-card-title>
        <v-card-text>
          <ResourceValueView label="certificate" :value="certificate" />
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
.summary-grid { display: grid; gap: 1rem; }
.summary-item { display: grid; gap: 0.4rem; }
.summary-label { color: #607286; font-size: 0.8rem; font-weight: 700; letter-spacing: 0.04em; text-transform: uppercase; }
.summary-value { color: #223748; line-height: 1.7; word-break: break-word; }
.chip-wrap { display: flex; flex-wrap: wrap; gap: 0.45rem; }
.empty-copy { color: #66788a; }
@media (max-width: 960px) { .detail-header { flex-direction: column; align-items: stretch; } }
</style>

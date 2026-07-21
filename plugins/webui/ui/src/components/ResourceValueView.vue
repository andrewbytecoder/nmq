<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  label?: string
  value: unknown
}>()

const isPrimitive = computed(
  () =>
    props.value === null ||
    typeof props.value === 'string' ||
    typeof props.value === 'number' ||
    typeof props.value === 'boolean',
)

const isArray = computed(() => Array.isArray(props.value))
const isObject = computed(() => typeof props.value === 'object' && props.value !== null && !Array.isArray(props.value))

const objectEntries = computed(() => (isObject.value ? Object.entries(props.value as Record<string, unknown>) : []))

function formatLabel(value: string) {
  return value
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/[_-]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/^./, (char) => char.toUpperCase())
}
</script>

<template>
  <div class="value-view">
    <div v-if="label" class="value-label">{{ formatLabel(label) }}</div>

    <div v-if="isPrimitive" class="primitive-value">
      {{ value === null ? 'null' : String(value) }}
    </div>

    <div v-else-if="isArray" class="array-value">
      <template v-if="(value as unknown[]).every((entry) => entry === null || ['string', 'number', 'boolean'].includes(typeof entry))">
        <div class="chip-wrap">
          <v-chip v-for="entry in (value as unknown[])" :key="String(entry)" size="small" variant="outlined">
            {{ String(entry) }}
          </v-chip>
        </div>
      </template>
      <template v-else>
        <v-expansion-panels variant="accordion">
          <v-expansion-panel v-for="(entry, index) in (value as unknown[])" :key="index">
            <v-expansion-panel-title>Item {{ index + 1 }}</v-expansion-panel-title>
            <v-expansion-panel-text>
              <ResourceValueView :value="entry" />
            </v-expansion-panel-text>
          </v-expansion-panel>
        </v-expansion-panels>
      </template>
    </div>

    <div v-else-if="isObject" class="object-value">
      <v-sheet rounded="lg" border class="object-sheet">
        <div v-for="[key, entry] in objectEntries" :key="key" class="object-row">
          <ResourceValueView :label="key" :value="entry" />
        </div>
      </v-sheet>
    </div>

    <div v-else class="primitive-value">
      {{ String(value) }}
    </div>
  </div>
</template>

<style scoped>
.value-view {
  display: grid;
  gap: 0.5rem;
}

.value-label {
  color: #607286;
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.primitive-value {
  color: #223748;
  line-height: 1.7;
  word-break: break-word;
}

.chip-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.object-sheet {
  display: grid;
  gap: 0;
  overflow: hidden;
}

.object-row {
  padding: 0.85rem 1rem;
  border-bottom: 1px solid rgba(34, 55, 72, 0.08);
}

.object-row:last-child {
  border-bottom: none;
}
</style>

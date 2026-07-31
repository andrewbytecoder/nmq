<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RemoteDriver, VueFinder, type DirEntry, type Theme as VueFinderTheme, useVueFinder } from 'vuefinder'
import 'vuefinder/dist/style.css'
import { useTheme } from 'vuetify'

import { apiBasePath, fetchJSON } from '../../lib/api'

type FileViewMeta = {
  root: string
}

type ThemeMode = 'auto' | VueFinderTheme

const fileViewRootPath = 'workdir://'

const vueFinderThemes: Array<{ title: string; value: VueFinderTheme }> = [
  { title: 'Silver', value: 'silver' },
  { title: 'Valorite', value: 'valorite' },
  { title: 'Midnight', value: 'midnight' },
  { title: 'Latte', value: 'latte' },
  { title: 'Rose', value: 'rose' },
  { title: 'Mythril', value: 'mythril' },
  { title: 'Lime', value: 'lime' },
  { title: 'Sky', value: 'sky' },
  { title: 'Ocean', value: 'ocean' },
  { title: 'Palenight', value: 'palenight' },
  { title: 'Arctic', value: 'arctic' },
  { title: 'Code', value: 'code' },
]

const finderId = 'ncp-fileview'
const driver = new RemoteDriver({
  baseURL: `${apiBasePath()}/fileview`,
  url: {
    list: '/list',
    delete: '/delete',
    rename: '/rename',
    createFile: '/create-file',
    createFolder: '/create-folder',
    upload: '/upload',
    preview: '/preview',
    download: '/download',
    search: '/search',
    save: '/save',
  },
})

const vuetifyTheme = useTheme()
const finderReady = ref(false)
const workdirRoot = ref('')
const currentPath = ref(fileViewRootPath)
const themeMode = ref<ThemeMode>('auto')
const uploadInput = ref<HTMLInputElement | null>(null)

const isDark = computed(() => vuetifyTheme.global.current.value.dark)
const effectiveFinderTheme = computed<VueFinderTheme>(() =>
  themeMode.value === 'auto' ? (isDark.value ? 'midnight' : 'silver') : themeMode.value,
)
const finderConfig = computed(() => ({
  path: currentPath.value,
  theme: effectiveFinderTheme.value,
  showMenuBar: true,
  showToolbar: true,
  showTreeView: true,
  notificationPosition: 'bottom-right' as const,
  loadingIndicator: 'linear' as const,
}))
const finderFeatures = {
  edit: true,
  newfile: true,
  newfolder: true,
  preview: true,
  search: true,
  rename: true,
  delete: true,
  download: true,
  history: true,
  fullscreen: true,
  upload: true,
  archive: false,
  unarchive: false,
  move: false,
  copy: false,
  language: false,
  theme: false,
  pinned: false,
}
const themeItems = computed(() => [
  { title: 'Auto (Follow App Theme)', value: 'auto' },
  ...vueFinderThemes,
])
const viewerKey = computed(() => `${effectiveFinderTheme.value}`)
const themeCaption = computed(() =>
  themeMode.value === 'auto' ? `Auto (${isDark.value ? 'midnight' : 'silver'})` : themeMode.value,
)

onMounted(async () => {
  try {
    const meta = await fetchJSON<FileViewMeta>('/fileview/meta')
    workdirRoot.value = meta.root
  } catch {
    workdirRoot.value = 'workdir'
  }
})

function handlePathChange(path: string) {
  currentPath.value = path
}

function handleReady() {
  finderReady.value = true
}

function getFinder() {
  if (!finderReady.value) {
    return null
  }

  try {
    return useVueFinder(finderId)
  } catch {
    return null
  }
}

function iconForItem(item: DirEntry) {
  if (item.type === 'dir') {
    return 'mdi-folder'
  }

  const ext = item.extension?.toLowerCase() || ''
  const mime = item.mime_type?.toLowerCase() || ''

  if (mime.startsWith('image/')) {
    return 'mdi-file-image-outline'
  }
  if (mime.startsWith('video/')) {
    return 'mdi-file-video-outline'
  }
  if (mime.startsWith('audio/')) {
    return 'mdi-file-music-outline'
  }
  if (mime === 'application/pdf' || ext === 'pdf') {
    return 'mdi-file-pdf-box'
  }
  if (['ts', 'tsx', 'js', 'jsx', 'vue', 'go', 'py', 'java', 'c', 'cpp', 'h', 'hpp'].includes(ext)) {
    return 'mdi-file-code-outline'
  }
  if (['json', 'yaml', 'yml', 'toml', 'xml'].includes(ext)) {
    return 'mdi-file-cog-outline'
  }
  if (['md', 'txt', 'log'].includes(ext) || mime.startsWith('text/')) {
    return 'mdi-file-document-outline'
  }
  if (['zip', 'tar', 'gz', '7z'].includes(ext)) {
    return 'mdi-zip-box-outline'
  }

  return 'mdi-file-outline'
}

function iconColor(item: DirEntry) {
  if (item.type === 'dir') {
    return 'primary'
  }

  const ext = item.extension?.toLowerCase() || ''
  if (ext === 'vue') {
    return 'success'
  }
  if (['ts', 'tsx', 'js', 'jsx'].includes(ext)) {
    return 'warning'
  }
  if (ext === 'go') {
    return 'info'
  }

  return 'secondary'
}

function openSelected(selected: DirEntry[]) {
  const finder = getFinder()
  if (!finder || !selected.length) {
    return
  }

  const first = selected[0]
  if (first.type === 'dir') {
    void finder.open(first.path)
    return
  }

  finder.preview(first.path)
}

function clearSelection() {
  const finder = getFinder()
  if (!finder) {
    return
  }

  finder.clearSelection()
}

function openUploadPicker() {
  uploadInput.value?.click()
}

async function uploadFiles(files: FileList | null) {
  if (!files?.length) {
    return
  }

  for (const file of Array.from(files)) {
    const formData = new FormData()
    formData.append('path', currentPath.value || fileViewRootPath)
    formData.append('file', file)

    await fetch(`${apiBasePath()}/fileview/upload`, {
      method: 'POST',
      body: formData,
    })
  }

  const finder = getFinder()
  if (finder) {
    await finder.open(currentPath.value || fileViewRootPath)
  }

  if (uploadInput.value) {
    uploadInput.value.value = ''
  }
}
</script>

<template>
  <div class="page-shell" data-testid="fileview-page">
    <div class="page-header">
      <div>
        <div class="eyebrow">FileView</div>
        <h1 class="page-title">File Viewer</h1>
        <p class="page-description">
          Browse the current backend workdir, preview files, edit text-based content, and keep the file
          manager theme aligned with the existing dashboard or override it with a dedicated VueFinder theme.
        </p>
      </div>
      <v-chip color="success" variant="tonal" prepend-icon="mdi-file-cabinet">
        Workdir mounted
      </v-chip>
    </div>

    <v-card rounded="xl" variant="outlined">
      <v-card-text class="controls-grid">
        <v-select
          v-model="themeMode"
          :items="themeItems"
          label="File window theme"
          variant="outlined"
          density="comfortable"
          hide-details
        />
        <v-chip color="secondary" variant="tonal" prepend-icon="mdi-theme-light-dark">
          {{ themeCaption }}
        </v-chip>
        <v-chip color="primary" variant="tonal" prepend-icon="mdi-folder-network-outline">
          {{ workdirRoot || 'workdir' }}
        </v-chip>
        <v-btn color="primary" variant="tonal" prepend-icon="mdi-upload" @click="openUploadPicker">
          Upload Files
        </v-btn>
        <input ref="uploadInput" class="hidden-upload" type="file" multiple @change="uploadFiles(($event.target as HTMLInputElement).files)" />
      </v-card-text>
    </v-card>

    <v-card rounded="xl" variant="elevated" class="finder-card">
      <v-card-text class="finder-card__body">
        <VueFinder
          :id="finderId"
          :key="viewerKey"
          :driver="driver"
          :config="finderConfig"
          :features="finderFeatures"
          locale="zhCN"
          selection-mode="multiple"
          @path-change="handlePathChange"
          @ready="handleReady"
        >
          <template #icon="{ item }">
            <v-icon :icon="iconForItem(item)" :color="iconColor(item)" size="20" />
          </template>

          <template #status-bar="{ path, count, selected }">
            <div class="status-bar">
              <div class="status-bar__meta">
                <v-chip size="small" variant="outlined" prepend-icon="mdi-folder-outline">
                  {{ path || '/' }}
                </v-chip>
                <v-chip size="small" variant="tonal" prepend-icon="mdi-checkbox-marked-circle-outline">
                  {{ count }} selected
                </v-chip>
              </div>
              <div class="status-bar__actions">
                <v-btn
                  size="small"
                  color="primary"
                  variant="tonal"
                  prepend-icon="mdi-open-in-app"
                  :disabled="!count"
                  @click="openSelected(selected)"
                >
                  Open Selected
                </v-btn>
                <v-btn
                  size="small"
                  variant="text"
                  prepend-icon="mdi-close-circle-outline"
                  :disabled="!count"
                  @click="clearSelection"
                >
                  Clear
                </v-btn>
              </div>
            </div>
          </template>
        </VueFinder>
      </v-card-text>
    </v-card>
  </div>
</template>

<style scoped>
.page-shell {
  display: grid;
  gap: 1.25rem;
  min-height: calc(100vh - 8rem);
}

.page-header {
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

.page-title {
  margin: 0;
  color: #16324f;
  font-size: clamp(1.9rem, 3vw, 2.8rem);
}

.page-description {
  max-width: 56rem;
  margin: 0.85rem 0 0;
  color: #54667a;
  line-height: 1.7;
}

.controls-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
}

.finder-card {
  overflow: hidden;
  flex: 1;
  min-height: 0;
}

.finder-card__body {
  padding: 0;
  display: flex;
  flex: 1;
}

.finder-card__body :deep(.vuefinder__themer) {
  width: 100%;
  height: clamp(34rem, calc(100vh - 18rem), 100vh);
  min-height: clamp(34rem, calc(100vh - 18rem), 100vh);
  border-radius: 1.5rem;
}

.finder-card__body :deep(.vuefinder__status-bar) {
  border-top: 1px solid rgba(22, 50, 79, 0.08);
}

.finder-card__body :deep(.vuefinder__main__fixed) {
  top: var(--app-bar-height, 64px) !important;
  left: var(--app-drawer-offset, 0px) !important;
  width: calc(100vw - var(--app-drawer-offset, 0px)) !important;
  height: calc(100vh - var(--app-bar-height, 64px)) !important;
  z-index: 2000 !important;
}

.status-bar {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  align-items: center;
  width: 100%;
  padding: 0.5rem 0.75rem;
}

.hidden-upload {
  display: none;
}

.status-bar__meta,
.status-bar__actions {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  flex-wrap: wrap;
}

@media (max-width: 960px) {
  .page-header,
  .status-bar {
    flex-direction: column;
    align-items: stretch;
  }

  .status-bar__actions {
    justify-content: flex-start;
  }

  .finder-card__body :deep(.vuefinder__themer) {
    height: clamp(28rem, calc(100vh - 20rem), 100vh);
    min-height: clamp(28rem, calc(100vh - 20rem), 100vh);
  }
}
</style>

let cachedBlobUrl: string | null = null

export function getHubDemoBlobUrl() {
  return cachedBlobUrl
}

export function setHubDemoBlobUrl(value: string | null) {
  cachedBlobUrl = value
}

export function resetHubDemoScriptCache() {
  cachedBlobUrl = null
}

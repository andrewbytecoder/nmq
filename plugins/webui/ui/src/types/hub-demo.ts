export type HubDemoRoute = {
  path: string
  label: string
  icon: string
  contentPath: string
  dynamicSegments?: string[]
  activeMatches?: string[]
}

export type HubDemoManifest = {
  routes: HubDemoRoute[]
}

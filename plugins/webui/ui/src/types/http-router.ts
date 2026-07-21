export type HttpRouterItem = {
  name: string
  service?: string
  rule?: string
  status: 'enabled' | 'disabled' | 'warning'
  provider: string
  using?: string[]
  entryPoints?: string[]
  tls?: Record<string, unknown>
  priority?: number
  priorityStr?: string
}

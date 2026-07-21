export type ResourceStats = {
  errors: number
  warnings: number
  total: number
}

export type EntryPointInfo = {
  name: string
  address: string
}

export type ProtocolOverview = {
  routers?: ResourceStats
  services?: ResourceStats
  middlewares?: ResourceStats
}

export type DashboardOverview = {
  http?: ProtocolOverview
  tcp?: ProtocolOverview
  udp?: ProtocolOverview
  features?: Record<string, boolean | string | number>
  providers?: string[]
}

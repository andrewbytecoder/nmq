export type HttpServiceItem = {
  name: string
  provider: string
  status: 'enabled' | 'disabled' | 'warning'
  type?: string
  usedBy?: string[]
  loadBalancer?: {
    servers?: { url: string }[]
  }
}

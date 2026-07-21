export type ResourceColumnType =
  | 'status'
  | 'text'
  | 'chips'
  | 'provider'
  | 'tls'
  | 'priority'
  | 'server-count'
  | 'date'
  | 'array-count'

export type ResourceColumn = {
  key: string
  label: string
  type: ResourceColumnType
}

export type ResourceConfig = {
  key: string
  routePath: string
  detailPath: string
  apiPath: string
  title: string
  section: string
  description: string
  detailDescription: string
  columns: ResourceColumn[]
}

export const resourceConfigs: ResourceConfig[] = [
  {
    key: 'http-routers',
    routePath: '/http/routers',
    detailPath: '/http/routers/:name',
    apiPath: '/http/routers',
    title: 'HTTP Routers',
    section: 'HTTP',
    description:
      'Browse HTTP routers, their rules, entrypoints, target services, providers, and routing priority.',
    detailDescription:
      'Inspect the selected HTTP router, including relationships and raw routing configuration payload.',
    columns: [
      { key: 'status', label: 'Status', type: 'status' },
      { key: 'tls', label: 'TLS', type: 'tls' },
      { key: 'rule', label: 'Rule', type: 'text' },
      { key: 'using', label: 'Entrypoints', type: 'chips' },
      { key: 'name', label: 'Name', type: 'text' },
      { key: 'service', label: 'Service', type: 'text' },
      { key: 'provider', label: 'Provider', type: 'provider' },
      { key: 'priority', label: 'Priority', type: 'priority' },
    ],
  },
  {
    key: 'http-services',
    routePath: '/http/services',
    detailPath: '/http/services/:name',
    apiPath: '/http/services',
    title: 'HTTP Services',
    section: 'HTTP',
    description:
      'Browse HTTP services, service type, backend server counts, providers, and continue into detailed payload inspection.',
    detailDescription:
      'Inspect the selected HTTP service, including backend servers, weighted targets, health checks, and raw service payload.',
    columns: [
      { key: 'status', label: 'Status', type: 'status' },
      { key: 'name', label: 'Name', type: 'text' },
      { key: 'type', label: 'Type', type: 'text' },
      { key: 'servers', label: 'Servers', type: 'server-count' },
      { key: 'provider', label: 'Provider', type: 'provider' },
    ],
  },
  {
    key: 'http-middlewares',
    routePath: '/http/middlewares',
    detailPath: '/http/middlewares/:name',
    apiPath: '/http/middlewares',
    title: 'HTTP Middlewares',
    section: 'HTTP',
    description:
      'Browse HTTP middlewares, linked routers, provider origin, and middleware type before drilling into details.',
    detailDescription:
      'Inspect the selected HTTP middleware, including linked routers, configuration payload, and any reported errors.',
    columns: [
      { key: 'status', label: 'Status', type: 'status' },
      { key: 'name', label: 'Name', type: 'text' },
      { key: 'type', label: 'Type', type: 'text' },
      { key: 'usedBy', label: 'Used By', type: 'chips' },
      { key: 'provider', label: 'Provider', type: 'provider' },
    ],
  },
  {
    key: 'tcp-routers',
    routePath: '/tcp/routers',
    detailPath: '/tcp/routers/:name',
    apiPath: '/tcp/routers',
    title: 'TCP Routers',
    section: 'TCP',
    description:
      'Browse TCP routers, SNI rules, entrypoints, TLS state, target services, and routing priority.',
    detailDescription:
      'Inspect the selected TCP router, including its entrypoints, TLS settings, linked middlewares, and raw payload.',
    columns: [
      { key: 'status', label: 'Status', type: 'status' },
      { key: 'tls', label: 'TLS', type: 'tls' },
      { key: 'rule', label: 'Rule', type: 'text' },
      { key: 'using', label: 'Entrypoints', type: 'chips' },
      { key: 'name', label: 'Name', type: 'text' },
      { key: 'service', label: 'Service', type: 'text' },
      { key: 'provider', label: 'Provider', type: 'provider' },
      { key: 'priority', label: 'Priority', type: 'priority' },
    ],
  },
  {
    key: 'tcp-services',
    routePath: '/tcp/services',
    detailPath: '/tcp/services/:name',
    apiPath: '/tcp/services',
    title: 'TCP Services',
    section: 'TCP',
    description:
      'Browse TCP services, load balancer types, backend counts, and protocol-specific service settings.',
    detailDescription:
      'Inspect the selected TCP service, including backend addresses, termination delay, and raw service payload.',
    columns: [
      { key: 'status', label: 'Status', type: 'status' },
      { key: 'name', label: 'Name', type: 'text' },
      { key: 'type', label: 'Type', type: 'text' },
      { key: 'servers', label: 'Servers', type: 'server-count' },
      { key: 'provider', label: 'Provider', type: 'provider' },
    ],
  },
  {
    key: 'tcp-middlewares',
    routePath: '/tcp/middlewares',
    detailPath: '/tcp/middlewares/:name',
    apiPath: '/tcp/middlewares',
    title: 'TCP Middlewares',
    section: 'TCP',
    description:
      'Browse TCP middlewares, linked routers, and provider origin while the React pages continue to be retired.',
    detailDescription:
      'Inspect the selected TCP middleware, linked resources, and raw middleware payload.',
    columns: [
      { key: 'status', label: 'Status', type: 'status' },
      { key: 'name', label: 'Name', type: 'text' },
      { key: 'type', label: 'Type', type: 'text' },
      { key: 'usedBy', label: 'Used By', type: 'chips' },
      { key: 'provider', label: 'Provider', type: 'provider' },
    ],
  },
  {
    key: 'udp-routers',
    routePath: '/udp/routers',
    detailPath: '/udp/routers/:name',
    apiPath: '/udp/routers',
    title: 'UDP Routers',
    section: 'UDP',
    description:
      'Browse UDP routers, entrypoints, target services, and route priority from the new Vue shell.',
    detailDescription:
      'Inspect the selected UDP router, including linked entrypoints, services, and raw router payload.',
    columns: [
      { key: 'status', label: 'Status', type: 'status' },
      { key: 'using', label: 'Entrypoints', type: 'chips' },
      { key: 'name', label: 'Name', type: 'text' },
      { key: 'service', label: 'Service', type: 'text' },
      { key: 'provider', label: 'Provider', type: 'provider' },
      { key: 'priority', label: 'Priority', type: 'priority' },
    ],
  },
  {
    key: 'udp-services',
    routePath: '/udp/services',
    detailPath: '/udp/services/:name',
    apiPath: '/udp/services',
    title: 'UDP Services',
    section: 'UDP',
    description:
      'Browse UDP services, backend counts, service type, and provider origin from the Vue migration shell.',
    detailDescription:
      'Inspect the selected UDP service, including backend addresses, weighted targets, and raw payload.',
    columns: [
      { key: 'status', label: 'Status', type: 'status' },
      { key: 'name', label: 'Name', type: 'text' },
      { key: 'type', label: 'Type', type: 'text' },
      { key: 'servers', label: 'Servers', type: 'server-count' },
      { key: 'provider', label: 'Provider', type: 'provider' },
    ],
  },
  {
    key: 'certificates',
    routePath: '/certificates',
    detailPath: '/certificates/:name',
    apiPath: '/certificates',
    title: 'Certificates',
    section: 'Certificates',
    description:
      'Browse certificate inventory, resolver origin, expiry, SAN coverage, issuer metadata, and status.',
    detailDescription:
      'Inspect the selected certificate, including issuer, SANs, fingerprints, validity period, and raw payload.',
    columns: [
      { key: 'status', label: 'Status', type: 'status' },
      { key: 'commonName', label: 'Common Name', type: 'text' },
      { key: 'resolver', label: 'Resolver', type: 'text' },
      { key: 'notAfter', label: 'Expires', type: 'date' },
      { key: 'sans', label: 'SANs', type: 'array-count' },
      { key: 'issuerCN', label: 'Issuer', type: 'text' },
    ],
  },
]

export function getResourceConfig(key: string) {
  return resourceConfigs.find((config) => config.key === key)
}

export function getResourceConfigByRoutePath(path: string) {
  return resourceConfigs.find((config) => config.routePath === path)
}

export type NavItem = {
  path: string
  label: string
  icon: string
  description: string
}

export type NavSection = {
  key: string
  label?: string
  items: NavItem[]
}

export const navSections: NavSection[] = [
  {
    key: 'dashboard',
    items: [
      {
        path: '/',
        label: 'Dashboard',
        icon: 'mdi-view-dashboard-outline',
        description: 'Overview of entrypoints, protocol resources, providers, and enabled features.',
      },
    ],
  },
  {
    key: 'http',
    label: 'HTTP',
    items: [
      {
        path: '/http/routers',
        label: 'HTTP Routers',
        icon: 'mdi-source-branch',
        description: 'Inspect HTTP routers and continue migrating list/detail pages from the legacy React UI.',
      },
      {
        path: '/http/services',
        label: 'HTTP Services',
        icon: 'mdi-server-network-outline',
        description: 'Inspect HTTP services, backend targets, and health information.',
      },
      {
        path: '/http/middlewares',
        label: 'HTTP Middlewares',
        icon: 'mdi-tune-variant',
        description: 'Inspect HTTP middlewares and their linked routers.',
      },
    ],
  },
  {
    key: 'tcp',
    label: 'TCP',
    items: [
      {
        path: '/tcp/routers',
        label: 'TCP Routers',
        icon: 'mdi-source-branch',
        description: 'Inspect TCP routers and their backend services.',
      },
      {
        path: '/tcp/services',
        label: 'TCP Services',
        icon: 'mdi-server-network-outline',
        description: 'Inspect TCP services, weighted targets, and protocol-specific settings.',
      },
      {
        path: '/tcp/middlewares',
        label: 'TCP Middlewares',
        icon: 'mdi-tune-variant',
        description: 'Inspect TCP middlewares and continue the Vue migration for detailed views.',
      },
    ],
  },
  {
    key: 'udp',
    label: 'UDP',
    items: [
      {
        path: '/udp/routers',
        label: 'UDP Routers',
        icon: 'mdi-source-branch',
        description: 'Inspect UDP routers and pending Vue rewrites for protocol resources.',
      },
      {
        path: '/udp/services',
        label: 'UDP Services',
        icon: 'mdi-server-network-outline',
        description: 'Inspect UDP services and datagram-oriented backend mappings.',
      },
    ],
  },
  {
    key: 'certificates',
    label: 'Certificates',
    items: [
      {
        path: '/certificates',
        label: 'Certificates',
        icon: 'mdi-certificate-outline',
        description: 'Inspect certificate inventory, expiry, and issuer metadata.',
      },
    ],
  },
  {
    key: 'storage',
    label: 'STORAGE',
    items: [
      {
        path: '/storage/sqlite',
        label: 'SQLITE',
        icon: 'mdi-database-outline',
        description: 'Browse dpcore SQLite tables, row counts, and key metadata from the embedded web UI.',
      },
    ],
  },
]

export const flatNavItems = navSections.flatMap((section) => section.items)

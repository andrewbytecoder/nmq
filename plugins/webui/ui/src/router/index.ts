import { createRouter, createWebHistory } from 'vue-router'

import CertificateDetailPage from '../pages/certificates/CertificateDetailPage.vue'
import CertificatesPage from '../pages/certificates/CertificatesPage.vue'
import DashboardPage from '../pages/DashboardPage.vue'
import GenericResourceCollectionPage from '../pages/GenericResourceCollectionPage.vue'
import GenericResourceDetailPage from '../pages/GenericResourceDetailPage.vue'
import HttpMiddlewareDetailPage from '../pages/http/HttpMiddlewareDetailPage.vue'
import HttpMiddlewaresPage from '../pages/http/HttpMiddlewaresPage.vue'
import HttpRouterDetailPage from '../pages/http/HttpRouterDetailPage.vue'
import HttpRoutersPage from '../pages/http/HttpRoutersPage.vue'
import HttpServiceDetailPage from '../pages/http/HttpServiceDetailPage.vue'
import HttpServicesPage from '../pages/http/HttpServicesPage.vue'
import HubDemoPage from '../pages/hub-demo/HubDemoPage.vue'
import SqliteTableDetailPage from '../pages/storage/SqliteTableDetailPage.vue'
import SqliteTablesPage from '../pages/storage/SqliteTablesPage.vue'
import TcpMiddlewareDetailPage from '../pages/tcp/TcpMiddlewareDetailPage.vue'
import TcpMiddlewaresPage from '../pages/tcp/TcpMiddlewaresPage.vue'
import TcpRouterDetailPage from '../pages/tcp/TcpRouterDetailPage.vue'
import TcpRoutersPage from '../pages/tcp/TcpRoutersPage.vue'
import TcpServiceDetailPage from '../pages/tcp/TcpServiceDetailPage.vue'
import TcpServicesPage from '../pages/tcp/TcpServicesPage.vue'
import UdpRouterDetailPage from '../pages/udp/UdpRouterDetailPage.vue'
import UdpRoutersPage from '../pages/udp/UdpRoutersPage.vue'
import UdpServiceDetailPage from '../pages/udp/UdpServiceDetailPage.vue'
import UdpServicesPage from '../pages/udp/UdpServicesPage.vue'
import { resourceConfigs } from '../resource-config'

export function resolveRouterBase(pathname?: string, envBase: string = import.meta.env.VITE_APP_BASE_URL || '') {
  if (envBase) {
    return envBase.endsWith('/') ? envBase : `${envBase}/`
  }

  const currentPathname = pathname ?? (typeof window !== 'undefined' ? window.location.pathname : '')

  if (currentPathname.endsWith('/dashboard')) {
    return `${currentPathname}/`
  }

  const dashboardMarker = '/dashboard/'
  const dashboardIndex = currentPathname.indexOf(dashboardMarker)
  if (dashboardIndex >= 0) {
    return currentPathname.slice(0, dashboardIndex + dashboardMarker.length)
  }

  return '/'
}

const base = resolveRouterBase()

const customCollectionPaths = new Set([
  '/http/routers',
  '/http/services',
  '/http/middlewares',
  '/certificates',
  '/tcp/routers',
  '/tcp/services',
  '/tcp/middlewares',
  '/udp/routers',
  '/udp/services',
])

const genericCollectionRoutes = resourceConfigs
  .filter((config) => !customCollectionPaths.has(config.routePath))
  .map((config) => ({
    path: config.routePath,
    name: config.key,
    component: GenericResourceCollectionPage,
    props: {
      resourceKey: config.key,
    },
    meta: {
      title: config.title,
    },
  }))

const detailRoutes = resourceConfigs.map((config) => ({
  path: config.detailPath,
  name: `${config.key}-detail`,
  component: GenericResourceDetailPage,
  props: {
    resourceKey: config.key,
  },
  meta: {
    title: `${config.title} Detail`,
  },
}))

export const router = createRouter({
  history: createWebHistory(base),
  routes: [
    {
      path: '/',
      name: 'dashboard',
      component: DashboardPage,
      meta: {
        title: 'Dashboard',
      },
    },
    {
      path: '/http/routers',
      name: 'http-routers',
      component: HttpRoutersPage,
      meta: {
        title: 'HTTP Routers',
      },
    },
    {
      path: '/http/routers/:name',
      name: 'http-router-detail',
      component: HttpRouterDetailPage,
      meta: {
        title: 'HTTP Router Detail',
      },
    },
    {
      path: '/http/services',
      name: 'http-services',
      component: HttpServicesPage,
      meta: {
        title: 'HTTP Services',
      },
    },
    {
      path: '/http/services/:name',
      name: 'http-service-detail',
      component: HttpServiceDetailPage,
      meta: {
        title: 'HTTP Service Detail',
      },
    },
    {
      path: '/http/middlewares',
      name: 'http-middlewares',
      component: HttpMiddlewaresPage,
      meta: {
        title: 'HTTP Middlewares',
      },
    },
    {
      path: '/http/middlewares/:name',
      name: 'http-middleware-detail',
      component: HttpMiddlewareDetailPage,
      meta: {
        title: 'HTTP Middleware Detail',
      },
    },
    {
      path: '/certificates',
      name: 'certificates',
      component: CertificatesPage,
      meta: {
        title: 'Certificates',
      },
    },
    {
      path: '/hub-dashboard',
      redirect: '/hub-dashboard/dashboard',
    },
    {
      path: '/hub-dashboard/:pathMatch(.*)*',
      name: 'hub-dashboard',
      component: HubDemoPage,
      meta: {
        title: 'Hub Demo',
      },
    },
    {
      path: '/tcp/routers',
      name: 'tcp-routers',
      component: TcpRoutersPage,
      meta: {
        title: 'TCP Routers',
      },
    },
    {
      path: '/tcp/routers/:name',
      name: 'tcp-router-detail',
      component: TcpRouterDetailPage,
      meta: {
        title: 'TCP Router Detail',
      },
    },
    {
      path: '/tcp/services',
      name: 'tcp-services',
      component: TcpServicesPage,
      meta: {
        title: 'TCP Services',
      },
    },
    {
      path: '/tcp/services/:name',
      name: 'tcp-service-detail',
      component: TcpServiceDetailPage,
      meta: {
        title: 'TCP Service Detail',
      },
    },
    {
      path: '/tcp/middlewares',
      name: 'tcp-middlewares',
      component: TcpMiddlewaresPage,
      meta: {
        title: 'TCP Middlewares',
      },
    },
    {
      path: '/tcp/middlewares/:name',
      name: 'tcp-middleware-detail',
      component: TcpMiddlewareDetailPage,
      meta: {
        title: 'TCP Middleware Detail',
      },
    },
    {
      path: '/udp/routers',
      name: 'udp-routers',
      component: UdpRoutersPage,
      meta: {
        title: 'UDP Routers',
      },
    },
    {
      path: '/udp/routers/:name',
      name: 'udp-router-detail',
      component: UdpRouterDetailPage,
      meta: {
        title: 'UDP Router Detail',
      },
    },
    {
      path: '/udp/services',
      name: 'udp-services',
      component: UdpServicesPage,
      meta: {
        title: 'UDP Services',
      },
    },
    {
      path: '/udp/services/:name',
      name: 'udp-service-detail',
      component: UdpServiceDetailPage,
      meta: {
        title: 'UDP Service Detail',
      },
    },
    {
      path: '/certificates/:name',
      name: 'certificate-detail',
      component: CertificateDetailPage,
      meta: {
        title: 'Certificate Detail',
      },
    },
    {
      path: '/storage/sqlite',
      name: 'storage-sqlite',
      component: SqliteTablesPage,
      meta: {
        title: 'SQLite Tables',
      },
    },
    {
      path: '/storage/sqlite/:tableName',
      name: 'storage-sqlite-detail',
      component: SqliteTableDetailPage,
      meta: {
        title: 'SQLite Table Detail',
      },
    },
    ...genericCollectionRoutes,
    ...detailRoutes.filter(
      (route) =>
        ![
          'http-routers-detail',
          'http-services-detail',
          'http-middlewares-detail',
          'tcp-routers-detail',
          'tcp-services-detail',
          'tcp-middlewares-detail',
          'udp-routers-detail',
          'udp-services-detail',
          'certificates-detail',
        ].includes(route.name),
    ),
  ],
})

router.afterEach((to) => {
  const title = typeof to.meta.title === 'string' ? to.meta.title : 'NCP'
  document.title = `${title} | NCP`
})

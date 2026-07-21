import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { vuetify } from '../../plugins/vuetify'

import TcpMiddlewareDetailPage from './TcpMiddlewareDetailPage.vue'
import TcpRouterDetailPage from './TcpRouterDetailPage.vue'
import TcpServiceDetailPage from './TcpServiceDetailPage.vue'

describe('TcpDetailPages', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })

  test('renders router-specific sections', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/tcp/routers/:name', component: TcpRouterDetailPage },
        { path: '/tcp/services/:name', component: TcpServiceDetailPage },
        { path: '/tcp/middlewares/:name', component: TcpMiddlewareDetailPage },
      ],
    })

    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const path = typeof input === 'string' ? input : input.toString()

      if (path.includes('/api/tcp/routers/')) {
        return new Response(
          JSON.stringify({
            name: 'tcp-all@docker',
            service: 'tcp-all',
            rule: 'HostSNI(`*`)',
            status: 'enabled',
            provider: 'docker',
            using: ['web-secured', 'web'],
            middlewares: ['test-inflightconn', 'test-ipwhitelist'],
            priority: 10,
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        )
      }

      if (path.endsWith('/api/entrypoints')) {
        return new Response(
          JSON.stringify([
            { name: 'web-secured', address: ':443' },
            { name: 'web', address: ':8000' },
          ]),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        )
      }

      return new Response('{}', { status: 404 })
    })

    router.push('/tcp/routers/tcp-all@docker')
    await router.isReady()

    const wrapper = mount(TcpRouterDetailPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('SNI Rule')
    expect(wrapper.html()).toContain('test-inflightconn')
    expect(wrapper.html()).toContain(':443')
    expect(wrapper.html()).toContain('tcp-all')
  })

  test('renders service-specific sections', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/tcp/services/:name', component: TcpServiceDetailPage },
        { path: '/tcp/routers/:name', component: TcpRouterDetailPage },
      ],
    })

    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          name: 'tcp-all@docker',
          status: 'enabled',
          provider: 'docker',
          type: 'loadbalancer',
          usedBy: ['tcp-all@docker'],
          loadBalancer: {
            terminationDelay: 10,
            servers: [{ address: '10.0.1.14:8080', weight: 3 }],
            healthCheck: {
              interval: '30s',
              timeout: '10s',
              port: 8080,
              send: 'PING',
              expect: 'PONG',
            },
          },
          serverStatus: {
            '10.0.1.14:8080': 'UP',
          },
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )

    router.push('/tcp/services/tcp-all@docker')
    await router.isReady()

    const wrapper = mount(TcpServiceDetailPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('Backend Servers')
    expect(wrapper.html()).toContain('Termination delay')
    expect(wrapper.html()).toContain('PING')
    expect(wrapper.html()).toContain('tcp-all@docker')
  })

  test('renders middleware-specific sections', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/tcp/middlewares/:name', component: TcpMiddlewareDetailPage },
        { path: '/tcp/routers/:name', component: TcpRouterDetailPage },
      ],
    })

    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          name: 'test-inflightconn',
          status: 'enabled',
          provider: 'docker',
          type: 'inflightconn',
          usedBy: ['tcp-all@docker'],
          inFlightConn: {
            amount: 10,
          },
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )

    router.push('/tcp/middlewares/test-inflightconn')
    await router.isReady()

    const wrapper = mount(TcpMiddlewareDetailPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('Middleware Configuration')
    expect(wrapper.html()).toContain('Amount')
    expect(wrapper.html()).toContain('10')
    expect(wrapper.html()).toContain('tcp-all@docker')
  })
})

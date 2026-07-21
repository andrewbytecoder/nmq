import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { vuetify } from '../../plugins/vuetify'

import TcpMiddlewareDetailPage from './TcpMiddlewareDetailPage.vue'
import TcpMiddlewaresPage from './TcpMiddlewaresPage.vue'
import TcpRouterDetailPage from './TcpRouterDetailPage.vue'
import TcpRoutersPage from './TcpRoutersPage.vue'
import TcpServiceDetailPage from './TcpServiceDetailPage.vue'
import TcpServicesPage from './TcpServicesPage.vue'

describe('TcpCollections', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })

  test('renders tcp routers collection', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/tcp/routers', component: TcpRoutersPage },
        { path: '/tcp/routers/:name', component: TcpRouterDetailPage },
      ],
    })

    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            name: 'tcp-all@docker',
            service: 'tcp-all',
            rule: 'HostSNI(`*`)',
            status: 'enabled',
            using: ['web-secured', 'web'],
            priority: 10,
            provider: 'docker',
          },
        ]),
        { status: 200, headers: { 'Content-Type': 'application/json', 'X-Next-Page': '1' } },
      ),
    )

    router.push('/tcp/routers')
    await router.isReady()

    const wrapper = mount(TcpRoutersPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('TCP Routers')
    expect(wrapper.html()).toContain('tcp-all@docker')
  })

  test('renders tcp services collection', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/tcp/services', component: TcpServicesPage },
        { path: '/tcp/services/:name', component: TcpServiceDetailPage },
      ],
    })

    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            name: 'tcp-all@docker',
            status: 'enabled',
            provider: 'docker',
            type: 'loadbalancer',
            loadBalancer: { servers: [{ address: '10.0.1.14:8080' }] },
          },
        ]),
        { status: 200, headers: { 'Content-Type': 'application/json', 'X-Next-Page': '1' } },
      ),
    )

    router.push('/tcp/services')
    await router.isReady()

    const wrapper = mount(TcpServicesPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('TCP Services')
    expect(wrapper.html()).toContain('tcp-all@docker')
    expect(wrapper.html()).toContain('loadbalancer')
  })

  test('renders tcp middlewares collection', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/tcp/middlewares', component: TcpMiddlewaresPage },
        { path: '/tcp/middlewares/:name', component: TcpMiddlewareDetailPage },
      ],
    })

    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            name: 'test-inflightconn',
            status: 'enabled',
            provider: 'docker',
            type: 'inflightconn',
            usedBy: ['tcp-all@docker'],
          },
        ]),
        { status: 200, headers: { 'Content-Type': 'application/json', 'X-Next-Page': '1' } },
      ),
    )

    router.push('/tcp/middlewares')
    await router.isReady()

    const wrapper = mount(TcpMiddlewaresPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('TCP Middlewares')
    expect(wrapper.html()).toContain('test-inflightconn')
    expect(wrapper.html()).toContain('inflightconn')
  })
})

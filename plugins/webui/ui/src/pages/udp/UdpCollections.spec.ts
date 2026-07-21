import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { vuetify } from '../../plugins/vuetify'

import UdpRouterDetailPage from './UdpRouterDetailPage.vue'
import UdpRoutersPage from './UdpRoutersPage.vue'
import UdpServiceDetailPage from './UdpServiceDetailPage.vue'
import UdpServicesPage from './UdpServicesPage.vue'

describe('UdpCollections', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })

  test('renders udp routers collection', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/udp/routers', component: UdpRoutersPage },
        { path: '/udp/routers/:name', component: UdpRouterDetailPage },
      ],
    })

    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            name: 'to-whoami-a@file',
            service: 'whoami@file',
            status: 'enabled',
            using: ['udp'],
            priority: 10,
            provider: 'file',
          },
        ]),
        { status: 200, headers: { 'Content-Type': 'application/json', 'X-Next-Page': '1' } },
      ),
    )

    router.push('/udp/routers')
    await router.isReady()

    const wrapper = mount(UdpRoutersPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('UDP Routers')
    expect(wrapper.html()).toContain('to-whoami-a@file')
  })

  test('renders udp services collection', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/udp/services', component: UdpServicesPage },
        { path: '/udp/services/:name', component: UdpServiceDetailPage },
      ],
    })

    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            name: 'whoami-a@file',
            status: 'enabled',
            provider: 'file',
            type: 'loadbalancer',
            loadBalancer: { servers: [{ address: '172.17.0.6:8080' }] },
          },
        ]),
        { status: 200, headers: { 'Content-Type': 'application/json', 'X-Next-Page': '1' } },
      ),
    )

    router.push('/udp/services')
    await router.isReady()

    const wrapper = mount(UdpServicesPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('UDP Services')
    expect(wrapper.html()).toContain('whoami-a@file')
    expect(wrapper.html()).toContain('loadbalancer')
  })
})

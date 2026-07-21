import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { vuetify } from '../../plugins/vuetify'

import UdpRouterDetailPage from './UdpRouterDetailPage.vue'
import UdpServiceDetailPage from './UdpServiceDetailPage.vue'

describe('UdpDetailPages', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })

  test('renders router-specific sections', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/udp/routers/:name', component: UdpRouterDetailPage },
        { path: '/udp/services/:name', component: UdpServiceDetailPage },
      ],
    })

    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const path = typeof input === 'string' ? input : input.toString()

      if (path.includes('/api/udp/routers/')) {
        return new Response(
          JSON.stringify({
            name: 'to-whoami-a@file',
            service: 'whoami@file',
            status: 'enabled',
            provider: 'file',
            using: ['udp'],
            priority: 10,
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        )
      }

      if (path.endsWith('/api/entrypoints')) {
        return new Response(
          JSON.stringify([{ name: 'udp', address: ':65535/udp' }]),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        )
      }

      return new Response('{}', { status: 404 })
    })

    router.push('/udp/routers/to-whoami-a@file')
    await router.isReady()

    const wrapper = mount(UdpRouterDetailPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('Entrypoints')
    expect(wrapper.html()).toContain(':65535/udp')
    expect(wrapper.html()).toContain('whoami@file')
  })

  test('renders service-specific sections', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/udp/services/:name', component: UdpServiceDetailPage },
        { path: '/udp/routers/:name', component: UdpRouterDetailPage },
      ],
    })

    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          name: 'whoami@file',
          status: 'enabled',
          provider: 'file',
          type: 'weighted',
          usedBy: ['to-whoami-a@file'],
          weighted: {
            services: [{ name: 'whoami-a', weight: 3 }],
          },
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )

    router.push('/udp/services/whoami@file')
    await router.isReady()

    const wrapper = mount(UdpServiceDetailPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('Weighted Targets')
    expect(wrapper.html()).toContain('whoami-a')
    expect(wrapper.html()).toContain('to-whoami-a@file')
  })
})

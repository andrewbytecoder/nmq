import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { vuetify } from '../../plugins/vuetify'
import GenericResourceDetailPage from '../GenericResourceDetailPage.vue'

import HttpServicesPage from './HttpServicesPage.vue'

describe('HttpServicesPage', () => {
  const fetchMock = vi.fn()

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/http/services', component: HttpServicesPage },
      {
        path: '/http/services/:name',
        component: GenericResourceDetailPage,
        props: { resourceKey: 'http-services' },
      },
    ],
  })

  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })

  test('renders service rows from the API', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            loadBalancer: { servers: [{ url: 'http://10.0.1.12:80' }] },
            status: 'enabled',
            name: 'api2_v2-example-beta1@docker',
            provider: 'docker',
            type: 'loadbalancer',
          },
          {
            weighted: { sticky: { cookie: {} } },
            status: 'enabled',
            name: 'canary2@file',
            provider: 'file',
            type: 'weighted',
          },
        ]),
        {
          status: 200,
          headers: {
            'Content-Type': 'application/json',
            'X-Next-Page': '1',
          },
        },
      ),
    )

    router.push('/http/services')
    await router.isReady()

    const wrapper = mount(HttpServicesPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/http/services?page=1&per_page=10&sortBy=name&direction=asc',
      expect.any(Object),
    )
    expect(wrapper.html()).toContain('HTTP Services')
    expect(wrapper.html()).toContain('api2_v2-example-beta1@docker')
    expect(wrapper.html()).toContain('loadbalancer')
    expect(wrapper.html()).toContain('canary2@file')
    expect(wrapper.html()).toContain('weighted')
  })
})

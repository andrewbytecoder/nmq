import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { vuetify } from '../../plugins/vuetify'
import GenericResourceDetailPage from '../GenericResourceDetailPage.vue'

import HttpRoutersPage from './HttpRoutersPage.vue'

describe('HttpRoutersPage', () => {
  const fetchMock = vi.fn()

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/http/routers', component: HttpRoutersPage },
      { path: '/http/routers/:name', component: GenericResourceDetailPage, props: { resourceKey: 'http-routers' } },
    ],
  })

  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })

  test('renders router rows from the API', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            name: 'server-secured@docker',
            service: 'api2_v2-example-beta1',
            rule: 'Host(`server`)',
            tls: {},
            status: 'enabled',
            using: ['web-secured'],
            provider: 'docker',
          },
          {
            name: 'orphan-router@file',
            service: 'unexistingservice',
            rule: 'Path(`somethingreallyunexpected`)',
            status: 'disabled',
            using: ['web'],
            provider: 'file',
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

    router.push('/http/routers')
    await router.isReady()

    const wrapper = mount(HttpRoutersPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/http/routers?page=1&per_page=10&sortBy=name&direction=asc',
      expect.any(Object),
    )
    expect(wrapper.html()).toContain('HTTP Routers')
    expect(wrapper.html()).toContain('server-secured@docker')
    expect(wrapper.html()).toContain('orphan-router@file')
    expect(wrapper.html()).toContain('web-secured')
  })
})

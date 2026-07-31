import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { vuetify } from '../../plugins/vuetify'

import ApiHandlersPage from './ApiHandlersPage.vue'

describe('ApiHandlersPage', () => {
  const fetchMock = vi.fn()

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/api/http-handlers', component: ApiHandlersPage, props: { protocol: 'routers' } }],
  })

  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })

  test('renders HTTP handler rows from the API', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            method: 'GET',
            path: '/healthz',
            handler: 'HealthHandler',
          },
          {
            method: 'POST',
            path: '/api/v1/login',
            handler: 'LoginHandler',
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

    router.push('/api/http-handlers')
    await router.isReady()

    const wrapper = mount(ApiHandlersPage, {
      props: {
        protocol: 'routers',
      },
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/handlers/routers?page=1&per_page=10&sortBy=path&direction=asc',
      expect.any(Object),
    )
    expect(wrapper.html()).toContain('ROUTERS Handler')
    expect(wrapper.html()).toContain('/healthz')
    expect(wrapper.html()).toContain('LoginHandler')
  })
})

import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { vuetify } from '../../plugins/vuetify'

import HttpMiddlewareDetailPage from './HttpMiddlewareDetailPage.vue'
import HttpMiddlewaresPage from './HttpMiddlewaresPage.vue'

describe('HttpMiddlewaresPage', () => {
  const fetchMock = vi.fn()

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/http/middlewares', component: HttpMiddlewaresPage },
      { path: '/http/middlewares/:name', component: HttpMiddlewareDetailPage },
    ],
  })

  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })

  test('renders middleware rows from the API', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            addPrefix: { prefix: '/foo' },
            status: 'enabled',
            usedBy: ['web@docker'],
            name: 'add-foo@docker',
            provider: 'docker',
            type: 'addprefix',
          },
          {
            plugin: { jwtAuth: {} },
            status: 'enabled',
            name: 'middleware-plugin@docker',
            provider: 'docker',
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

    router.push('/http/middlewares')
    await router.isReady()

    const wrapper = mount(HttpMiddlewaresPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/http/middlewares?page=1&per_page=10&sortBy=name&direction=asc',
      expect.any(Object),
    )
    expect(wrapper.html()).toContain('HTTP Middlewares')
    expect(wrapper.html()).toContain('add-foo@docker')
    expect(wrapper.html()).toContain('addprefix')
    expect(wrapper.html()).toContain('middleware-plugin@docker')
    expect(wrapper.html()).toContain('jwtAuth')
  })
})

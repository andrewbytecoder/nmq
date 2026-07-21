import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { vuetify } from '../../plugins/vuetify'

import HttpMiddlewareDetailPage from './HttpMiddlewareDetailPage.vue'
import HttpRouterDetailPage from './HttpRouterDetailPage.vue'
import HttpServiceDetailPage from './HttpServiceDetailPage.vue'

describe('HttpDetailPages', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })

  test('renders router-specific sections', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/http/routers/:name', component: HttpRouterDetailPage },
        { path: '/http/services/:name', component: HttpServiceDetailPage },
        { path: '/http/middlewares/:name', component: HttpMiddlewareDetailPage },
      ],
    })

    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const path = typeof input === 'string' ? input : input.toString()

      if (path.includes('/api/http/routers/')) {
        return new Response(
          JSON.stringify({
            name: 'orphan-router@file',
            service: 'unexistingservice',
            rule: 'Path(`somethingreallyunexpectedbutalsoverylongitgetsoutofthecontainermaybe`)',
            status: 'disabled',
            provider: 'file',
            using: ['web-secured', 'web'],
            middlewares: ['middleware00@docker', 'middleware01@docker'],
            priority: 30,
            tls: {
              options: 'foo@file',
              certResolver: 'acme-dns-challenge',
              domains: [{ main: 'example.com', sans: ['foo.example.com'] }],
            },
            error: ['router error'],
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

    router.push('/http/routers/orphan-router@file')
    await router.isReady()

    const wrapper = mount(HttpRouterDetailPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('Routing Rule')
    expect(wrapper.html()).toContain('middleware00@docker')
    expect(wrapper.html()).toContain('foo.example.com')
    expect(wrapper.html()).toContain('router error')
  })

  test('renders service-specific sections', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/http/services/:name', component: HttpServiceDetailPage },
        { path: '/http/routers/:name', component: HttpRouterDetailPage },
      ],
    })

    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          name: 'api3_v2-example-beta1@docker',
          status: 'enabled',
          provider: 'docker',
          type: 'mirroring',
          usedBy: ['server-mtls@docker'],
          loadBalancer: {
            passHostHeader: true,
            servers: [{ url: 'http://10.0.1.20:80' }],
            healthCheck: {
              scheme: 'https',
              path: '/health',
              interval: '5s',
            },
          },
          serverStatus: {
            'http://10.0.1.20:80': 'UP',
          },
          mirroring: {
            service: 'one@docker',
            mirrors: [{ name: 'two@docker', percent: 10 }],
          },
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )

    router.push('/http/services/api3_v2-example-beta1@docker')
    await router.isReady()

    const wrapper = mount(HttpServiceDetailPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('Backend Servers')
    expect(wrapper.html()).toContain('Health Check')
    expect(wrapper.html()).toContain('Mirrors')
    expect(wrapper.html()).toContain('server-mtls@docker')
  })

  test('renders middleware-specific sections', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/http/middlewares/:name', component: HttpMiddlewareDetailPage },
        { path: '/http/routers/:name', component: HttpRouterDetailPage },
      ],
    })

    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          name: 'add-foo@docker',
          status: 'enabled',
          provider: 'docker',
          type: 'addprefix',
          usedBy: ['web@docker'],
          addPrefix: {
            prefix: '/path',
          },
          error: ['message 1'],
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )

    router.push('/http/middlewares/add-foo@docker')
    await router.isReady()

    const wrapper = mount(HttpMiddlewareDetailPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('Middleware Configuration')
    expect(wrapper.html()).toContain('/path')
    expect(wrapper.html()).toContain('web@docker')
    expect(wrapper.html()).toContain('message 1')
  })
})

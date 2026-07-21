import { mount } from '@vue/test-utils'
import { describe, expect, test } from 'vitest'

import App from './App.vue'
import { vuetify } from './plugins/vuetify'
import { resolveRouterBase, router } from './router'

describe('App', () => {
  test('detects the dashboard base path from the current URL', () => {
    expect(resolveRouterBase('/dashboard/')).toBe('/dashboard/')
    expect(resolveRouterBase('/dashboard/http/routers')).toBe('/dashboard/')
    expect(resolveRouterBase('/proxy/dashboard/http/routers')).toBe('/proxy/dashboard/')
    expect(resolveRouterBase('/dashboard')).toBe('/dashboard/')
    expect(resolveRouterBase('/', '/custom/base')).toBe('/custom/base/')
  })

  test('renders the main dashboard shell', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = typeof input === 'string' ? input : input.toString()

        if (path.endsWith('/api/entrypoints')) {
          return {
            ok: true,
            json: async () => [{ name: 'web', address: ':80' }],
          } as Response
        }

        if (path.endsWith('/api/overview')) {
          return {
            ok: true,
            json: async () => ({
              http: {
                routers: { total: 3, warnings: 0, errors: 0 },
                services: { total: 2, warnings: 0, errors: 0 },
                middlewares: { total: 1, warnings: 0, errors: 0 },
              },
              tcp: {
                routers: { total: 1, warnings: 0, errors: 0 },
                services: { total: 1, warnings: 0, errors: 0 },
                middlewares: { total: 0, warnings: 0, errors: 0 },
              },
              udp: {
                routers: { total: 1, warnings: 0, errors: 0 },
                services: { total: 1, warnings: 0, errors: 0 },
              },
              features: {
                metrics: true,
              },
              providers: ['docker'],
            }),
          } as Response
        }

        return {
          ok: false,
          status: 404,
          json: async () => ({}),
        } as Response
      }),
    )

    router.push('/')
    await router.isReady()

    const wrapper = mount(App, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(wrapper.html()).toContain('NCP')
    expect(wrapper.html()).toContain('Dashboard')
    expect(wrapper.html()).toContain('Entrypoints')
    expect(wrapper.html()).toContain('docker')
  })
})

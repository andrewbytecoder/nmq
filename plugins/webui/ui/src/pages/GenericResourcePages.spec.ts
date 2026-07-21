import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { vuetify } from '../plugins/vuetify'

import GenericResourceCollectionPage from './GenericResourceCollectionPage.vue'
import GenericResourceDetailPage from './GenericResourceDetailPage.vue'

describe('GenericResourcePages', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })

  test('renders TCP middlewares through the generic collection page', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/tcp/middlewares', component: GenericResourceCollectionPage, props: { resourceKey: 'tcp-middlewares' } },
        { path: '/tcp/middlewares/:name', component: GenericResourceDetailPage, props: { resourceKey: 'tcp-middlewares' } },
      ],
    })

    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            status: 'enabled',
            usedBy: ['tcp-all@docker'],
            name: 'test-inflightconn',
            type: 'inflightconn',
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

    router.push('/tcp/middlewares')
    await router.isReady()

    const wrapper = mount(GenericResourceCollectionPage, {
      props: { resourceKey: 'tcp-middlewares' },
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('TCP Middlewares')
    expect(wrapper.html()).toContain('test-inflightconn')
    expect(wrapper.html()).toContain('inflightconn')
  })

  test('renders certificate detail through the generic detail page', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/certificates/:name', component: GenericResourceDetailPage, props: { resourceKey: 'certificates' } }],
    })

    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          name: 'cert-a',
          commonName: 'example.com',
          status: 'enabled',
          resolver: 'letsencrypt',
          notAfter: '2026-06-15T10:00:00Z',
          sans: ['example.com', 'foo.example.com'],
          issuerCN: "Let's Encrypt Authority X3",
          signatureAlgorithm: 'SHA256-RSA',
        }),
        {
          status: 200,
          headers: {
            'Content-Type': 'application/json',
          },
        },
      ),
    )

    router.push('/certificates/cert-a')
    await router.isReady()

    const wrapper = mount(GenericResourceDetailPage, {
      props: { resourceKey: 'certificates' },
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('example.com')
    expect(wrapper.html()).toContain('letsencrypt')
    expect(wrapper.html()).toContain('SHA256-RSA')
  })
})

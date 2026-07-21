import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { vuetify } from '../../plugins/vuetify'

import CertificateDetailPage from './CertificateDetailPage.vue'
import CertificatesPage from './CertificatesPage.vue'

describe('CertificatesPage', () => {
  const fetchMock = vi.fn()

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/certificates', component: CertificatesPage },
      { path: '/certificates/:name', component: CertificateDetailPage },
    ],
  })

  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })

  test('renders certificate rows from the API', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            name: 'cert-a',
            commonName: 'example.com',
            sans: ['example.com', 'www.example.com'],
            issuerOrg: 'Acme Co',
            issuerCN: 'Acme Root CA',
            notAfter: new Date(Date.now() + 365 * 24 * 60 * 60 * 1000).toISOString(),
            notBefore: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString(),
            status: 'enabled',
          },
          {
            name: 'cert-b',
            commonName: 'expired.com',
            sans: ['expired.com'],
            issuerOrg: 'Expired CA',
            notAfter: new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString(),
            notBefore: new Date(Date.now() - 365 * 24 * 60 * 60 * 1000).toISOString(),
            status: 'expired',
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

    router.push('/certificates')
    await router.isReady()

    const wrapper = mount(CertificatesPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/certificates?page=1&per_page=10&sortBy=name&direction=asc',
      expect.any(Object),
    )
    expect(wrapper.html()).toContain('Certificates')
    expect(wrapper.html()).toContain('example.com')
    expect(wrapper.html()).toContain('Acme Co')
    expect(wrapper.html()).toContain('expired.com')
    expect(wrapper.html()).toContain('EXPIRED')
  })
})

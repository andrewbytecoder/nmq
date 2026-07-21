import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { vuetify } from '../../plugins/vuetify'

import CertificateDetailPage from './CertificateDetailPage.vue'

describe('CertificateDetailPage', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })

  test('renders certificate-specific sections', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/certificates/:name', component: CertificateDetailPage }],
    })

    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          name: 'cert-a',
          commonName: 'example.com',
          sans: ['example.com', 'www.example.com'],
          issuerOrg: 'Test CA',
          issuerCN: 'Test Root CA',
          notAfter: '2027-01-15T10:00:00Z',
          notBefore: '2024-01-15T10:00:00Z',
          status: 'enabled',
          serialNumber: '123456',
          version: '3',
          keyType: 'RSA',
          keySize: 2048,
          signatureAlgorithm: 'SHA256-RSA',
          certFingerprint: 'fingerprint-a',
          publicKeyFingerprint: 'fingerprint-b',
          resolver: 'letsencrypt',
          organization: 'Example Corp',
          country: 'US',
          issuerCountry: 'US',
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )

    router.push('/certificates/cert-a')
    await router.isReady()

    const wrapper = mount(CertificateDetailPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain('Issued To')
    expect(wrapper.html()).toContain('Issued By')
    expect(wrapper.html()).toContain('www.example.com')
    expect(wrapper.html()).toContain('SHA256-RSA')
    expect(wrapper.html()).toContain('fingerprint-a')
  })
})

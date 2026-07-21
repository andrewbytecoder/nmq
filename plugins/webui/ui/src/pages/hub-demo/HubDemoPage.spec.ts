import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { vuetify } from '../../plugins/vuetify'
import verifySignature from '../../utils/workers/scriptVerification'

vi.mock('../../utils/workers/scriptVerification', () => ({
  default: vi.fn(),
}))

import { resetHubDemoScriptCache } from './hubDemoCache'
import HubDemoPage from './HubDemoPage.vue'

describe('HubDemoPage', () => {
  const mockVerifySignature = vi.mocked(verifySignature)

  beforeEach(() => {
    mockVerifySignature.mockReset()
    resetHubDemoScriptCache()
    globalThis.URL.createObjectURL = vi.fn(() => 'blob:mock-url')
    globalThis.Worker = vi.fn() as unknown as typeof Worker
  })

  test('renders remote demo when signature is verified', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/hub-dashboard/:pathMatch(.*)*', component: HubDemoPage }],
    })

    mockVerifySignature.mockResolvedValue({
      verified: true,
      scriptContent: new ArrayBuffer(64),
    })

    router.push('/hub-dashboard/dashboard')
    await router.isReady()

    const wrapper = mount(HubDemoPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(mockVerifySignature).toHaveBeenCalled()
    expect(wrapper.html()).toContain('hub-ui-demo-app')
    expect(wrapper.html()).toContain('dashboard')
  })

  test('renders error state when signature verification fails', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/hub-dashboard/:pathMatch(.*)*', component: HubDemoPage }],
    })

    mockVerifySignature.mockResolvedValue({
      verified: false,
    })

    router.push('/hub-dashboard/dashboard')
    await router.isReady()

    const wrapper = mount(HubDemoPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(wrapper.html()).toContain("Oops! We couldn't load the demo content.")
  })
})

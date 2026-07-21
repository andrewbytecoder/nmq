import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { vuetify } from '../../plugins/vuetify'

import SqliteTableDetailPage from './SqliteTableDetailPage.vue'
import SqliteTablesPage from './SqliteTablesPage.vue'

describe('SQLite storage pages', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })

  test('renders table names as links to the detail page', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/storage/sqlite', name: 'storage-sqlite', component: SqliteTablesPage },
        { path: '/storage/sqlite/:tableName', name: 'storage-sqlite-detail', component: SqliteTableDetailPage },
      ],
    })

    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            name: 'cert_info',
            type: 'managed',
            rowCount: 2,
            columnCount: 4,
            primaryKeys: ['product_id', 'file_name'],
            primaryCount: 2,
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

    router.push('/storage/sqlite')
    await router.isReady()

    const wrapper = mount(SqliteTablesPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/storage/sqlite?page=1&per_page=10&sortBy=name&direction=asc',
      expect.any(Object),
    )
    expect(wrapper.html()).toContain('cert_info')
    expect(wrapper.html()).toContain('/storage/sqlite/cert_info')
  })

  test('renders detailed row and column data for a selected table', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/storage/sqlite', name: 'storage-sqlite', component: SqliteTablesPage },
        { path: '/storage/sqlite/:tableName', name: 'storage-sqlite-detail', component: SqliteTableDetailPage },
      ],
    })

    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          name: 'cert_info',
          type: 'managed',
          columns: ['product_id', 'file_name', 'md5', 'expire_timestamp'],
          primaryKeys: ['product_id', 'file_name'],
          rows: [
            {
              product_id: 'prod-a',
              file_name: 'tls.crt',
              md5: 'abc123',
              expire_timestamp: 1780000000,
            },
          ],
          rowCount: 1,
          page: 1,
          perPage: 20,
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )

    router.push('/storage/sqlite/cert_info')
    await router.isReady()

    const wrapper = mount(SqliteTableDetailPage, {
      global: {
        plugins: [router, vuetify],
      },
    })

    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith('/api/storage/sqlite/cert_info?page=1&per_page=20', expect.any(Object))
    expect(wrapper.html()).toContain('cert_info')
    expect(wrapper.html()).toContain('product_id')
    expect(wrapper.html()).toContain('tls.crt')
    expect(wrapper.html()).toContain('abc123')
  })
})

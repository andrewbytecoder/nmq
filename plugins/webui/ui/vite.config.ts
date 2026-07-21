/// <reference types="vitest" />
/// <reference types="vite/client" />

import vue from '@vitejs/plugin-vue'
import { defineConfig, loadEnv } from 'vite'
import vuetify, { transformAssetUrls } from 'vite-plugin-vuetify'
import viteTsconfigPaths from 'vite-tsconfig-paths'

export default ({ mode }: { mode: string }) => {
  process.env = { ...process.env, ...loadEnv(mode, process.cwd()) }

  return defineConfig({
    base: process.env.VITE_APP_BASE_URL || '',
    plugins: [
      vue({
        template: {
          transformAssetUrls,
          compilerOptions: {
            isCustomElement: (tag) => ['hub-ui-demo-app', 'hub-button-app'].includes(tag),
          },
        },
      }),
      vuetify({ autoImport: true }),
      viteTsconfigPaths(),
    ],
    server: {
      open: 'index.dev.html',
      port: 3000,
    },
    build: {
      emptyOutDir: true,
      outDir: './static',
    },
    test: {
      css: true,
      environment: 'jsdom',
      globals: true,
      pool: 'threads',
      setupFiles: './test/setup.ts',
      include: [
        'src/App.spec.ts',
        'src/pages/GenericResourcePages.spec.ts',
        'src/pages/certificates/CertificateDetailPage.spec.ts',
        'src/pages/certificates/CertificatesPage.vue.spec.ts',
        'src/pages/hub-demo/HubDemoPage.spec.ts',
        'src/pages/http/HttpDetailPages.spec.ts',
        'src/pages/http/HttpMiddlewaresPage.vue.spec.ts',
        'src/pages/http/HttpRoutersPage.vue.spec.ts',
        'src/pages/http/HttpServicesPage.vue.spec.ts',
        'src/pages/storage/SqliteStoragePages.spec.ts',
        'src/pages/tcp/TcpCollections.spec.ts',
        'src/pages/tcp/TcpDetailPages.spec.ts',
        'src/pages/udp/UdpCollections.spec.ts',
        'src/pages/udp/UdpDetailPages.spec.ts',
      ],
      server: {
        deps: {
          inline: ['vuetify', '@mdi/font'],
        },
      },
    },
  })
}

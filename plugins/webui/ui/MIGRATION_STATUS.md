# WebUI Migration Status

This document records the current state of the `webui/` migration from the legacy React implementation to the Vue 3 + TypeScript + Vuetify implementation.

## Current frontend runtime

The active frontend runtime now uses:

- Vue 3
- TypeScript
- Vuetify
- Vue Router
- Vite

The active application entrypoints are:

- `src/main.ts`
- `src/App.vue`
- `src/router/index.ts`

The WebUI still builds into `webui/static/`, which keeps the existing Go `embed` and image build workflow unchanged.

## Functional coverage migrated to Vue

The following user-facing features are now handled by Vue routes and Vue pages:

- Dashboard overview
- HTTP routers list
- HTTP services list
- HTTP middlewares list
- TCP routers list
- TCP services list
- TCP middlewares list
- UDP routers list
- UDP services list
- Certificates list
- HTTP router detail
- HTTP service detail
- HTTP middleware detail
- TCP router detail
- TCP service detail
- TCP middleware detail
- UDP router detail
- UDP service detail
- Certificate detail
- Hub demo entry route under `/hub-dashboard`

## Deliberately retained source files

These files remain because they are still used by the Vue implementation:

- `src/components/ResourceValueView.vue`
  Used by the generic and specialized detail pages to render nested payload structures.

- `src/composables/*.ts`
  Provide the active data-loading and routing logic for the Vue pages.

- `src/lib/api.ts`
  Centralized API and pagination fetch helpers used by the Vue pages.

- `src/libs/parsers.ts`
  Still used by `HTTP Middlewares` to resolve middleware type display.

- `src/utils/workers/scriptVerification.ts`
- `src/utils/workers/scriptVerificationWorker.ts`
- `src/utils/workers/scriptVerification.spec.ts`
- `src/utils/workers/scriptVerification.integration.spec.ts`
  Still required for the Hub demo signed remote script / manifest verification flow.

- `src/pages/hub-demo/constants.ts`
- `src/pages/hub-demo/hubDemoCache.ts`
- `src/pages/hub-demo/HubDemoPage.vue`
- `src/pages/hub-demo/HubDemoPage.spec.ts`
  These replace the legacy Hub demo route with a Vue implementation.

- `src/types/*.d.ts` and `src/types/*.ts`
  Some declarations are still used by helper utilities and page typing.

## Legacy React code removed

The main React application entry and page tree have been removed, including:

- `src/index.tsx`
- `src/App.tsx`
- `src/routes.tsx`
- the old `src/pages/**/*.{tsx}` page implementations
- the old React tests for those pages
- old React-only hooks, contexts, and layout files that were no longer referenced

## Remaining cleanup opportunities

The migration is functionally complete for the main routes, but some small cleanup opportunities remain:

- Audit `src/types/` and remove declarations that no longer serve the Vue codepath.
- Audit `src/libs/parsers.ts` and inline it if desired.
- Revisit `jsdom` stylesheet warnings during tests.

## Verification status

As of July 20, 2026, the Vue codepath passes:

- `corepack yarn lint`
- `corepack yarn test`
- `corepack yarn tsc`
- `corepack yarn build`

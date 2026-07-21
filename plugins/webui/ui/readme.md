# Traefik Web UI

Access to Traefik Web UI, ex: http://localhost:8080

## Interface

Traefik Web UI provides two main categories of information:

- Providers together with their routed resources.
- Health and topology information for the proxy instance.

## How to build (for backend developer)

Use the Makefile :

```shell
make binary-with-webui # Rebuild Vue assets and compile a Traefik binary with embedded WebUI.
make image-with-webui  # Rebuild Vue assets, compile the binary, and build a Docker image with embedded WebUI.
```

## Frontend stack

The current WebUI implementation uses:

- [Vue 3](https://vuejs.org/)
- [TypeScript](https://www.typescriptlang.org/)
- [Vuetify](https://vuetifyjs.com/)
- [Vue Router](https://router.vuejs.org/)
- [Vite](https://vitejs.dev/)
- [Vitest](https://vitest.dev/)

## How to build (only for frontend developer)

- prerequisite: [Node 22](https://nodejs.org) [npm](https://www.npmjs.com/)

- Go to the `webui/` directory

- To install dependencies, execute the following commands:

  - `npm install`

- Build static Web UI, execute the following command:

  - `npm run build`

- Static contents are built in the `webui/static/` directory

**Do not manually change the files in the `webui/static/` directory**

The build allows to:
  - optimize all JavaScript
  - optimize all CSS
  - add vendor prefixes to CSS (cross-browser support)
  - add a hash in the file names to prevent browser cache problems
  - optimize all images at build time
  - bundle JavaScript in one file

## How to edit (only for frontend developer)

**Do not manually change the files in the `webui/static/` directory**

- Go to the `webui/` directory
- Edit files in `webui/src/`
- Create and populate the `.env` file using the values inside `.env.sample` file.
- Run in development mode :
  - `npm run dev`
- The application will be available at `http://localhost:3000/`.

## How to run tests

- Execute the following commands:
  - `npm run test`
  - or `npm run test:watch` if you want them in watch mode

## Libraries

- [Node](https://nodejs.org)
- [npm](https://www.npmjs.com/)
- [Vue 3](https://vuejs.org/)
- [Vuetify](https://vuetifyjs.com/)
- [Vite](https://vitejs.dev/)
- [Vitest](https://vitest.dev/)

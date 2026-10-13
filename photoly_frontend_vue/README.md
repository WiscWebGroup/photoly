# photoly_frontend_vue

Vue 3 + TypeScript + Vite, styled with Tailwind CSS and shadcn-vue, state in Pinia, installable as a PWA.

## Recommended IDE Setup

[VSCode](https://code.visualstudio.com/) + [Vue (Official)](https://marketplace.visualstudio.com/items?itemName=Vue.volar), ESLint, and Prettier (see `.vscode/extensions.json`).

## Customize configuration

See [Vite Configuration Reference](https://vitejs.dev/config/).

## Project Setup

```sh
pnpm install
```

### Compile and Hot-Reload for Development

```sh
pnpm run dev
```

### Compile and Minify for Production

```sh
pnpm run build
```

### Lint, Type-Check, Format

```sh
pnpm lint
pnpm typecheck
pnpm format
```

### Tests

```sh
pnpm test            # Vitest unit/component tests (watch mode locally)
pnpm test:coverage
pnpm exec playwright install chromium webkit   # first time only
pnpm test:e2e        # Playwright; starts the dev server itself
```

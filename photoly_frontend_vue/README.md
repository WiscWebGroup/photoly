# Photoly frontend

Vue 3, TypeScript, Vite, Pinia, Tailwind CSS and shadcn-vue; installable as a PWA.

## Setup and API configuration

Use Node.js 26 (matching the Docker build; CI also validates Node.js 24) and pnpm 10.30.1 (pinned in `package.json`). Install pnpm if needed with `npm install -g pnpm@10.30.1`. Run these commands from `photoly_frontend_vue/`:

```sh
pnpm install --frozen-lockfile
pnpm dev
```

The tracked `.env` defaults `VITE_APP_BASE_URL` to `http://localhost:8384`. Set it in a local `.env.local` for development, or `.env.production.local` for a production build. Use the externally reachable backend URL when browsers run on another machine. Vite embeds this value at build time: changing the root Docker `.env` or a running container's environment does not update an already built frontend. Rebuild `web` after changing it. `PUBLIC_BASE_URL` separately controls backend-generated share URLs.

VS Code with Vue (Official), ESLint and Prettier is recommended; see `.vscode/extensions.json`.

## Build and checks

```sh
pnpm typecheck
pnpm lint
pnpm test --run
pnpm test:coverage
pnpm build
pnpm exec playwright install chromium webkit
pnpm test:e2e
```

`pnpm test` without `--run` starts watch mode. Playwright starts its own development server and covers Chromium, WebKit and mobile layouts using mocked APIs. Real backend/model browser tests use the separate opt-in configuration described in [AI operations](../docs/ai-backend-operations.md). `pnpm format` rewrites source/test formatting and is not a read-only check.

## User-facing features

- Smart Search defaults to a single search box. Text matches appear first; semantic results update the ranking when available. Previous search modes remain selectable.
- Photos and videos share the media viewer and similar recommendations. Results belong to each item and expire 30 seconds after leaving; Back to top also collapses them.
- Edit Avatar supports dragging, zoom and keyboard adjustment within a circular preview; the upload is a complete square 512 × 512 JPEG.
- Extract Text provides explicit recognition, editing, saving, copying and unsaved-change prompts. Recognition uses backend Tesseract, independently of model availability.
- Smart tags/galleries have icons, labels and distinct styling. Personal settings provide opt-in, progress, refresh and restoration; administrators control availability in AI Management.

See [deployment](../README.md), [OCR](../docs/ocr.md) and [Smart Tag](../docs/smart-tag.md) for backend requirements and limits.

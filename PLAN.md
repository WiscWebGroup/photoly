# Photoly frontend modernization

## Context
`photoly_frontend_vue` is Vue 3.4 + Vite 5 + Naive UI + vue-router 4 + Leaflet, plain JS, no lint/tests. 18 `.vue` files, almost all hybrid (tiny `<script setup>` of imports + a big Options-API `<script>`). Goal: a modern-looking, responsive, installable UI built on current Vue conventions.

Decisions (confirmed): responsive web + PWA; Tailwind + shadcn-vue (replace Naive UI); stay on Vite SPA (no Nuxt); adopt TypeScript incrementally.

Strategy: **touch each file once** — convert it to `<script setup lang="ts">` *and* restyle it with Tailwind/shadcn-vue in the same pass, rather than a separate "Composition API" sweep followed by a "redesign" sweep. Naive UI stays installed (coexisting) until the last file is converted, then is removed.

## Phase 0 — Branch & baseline
- `git checkout -b modernize-frontend` from clean `main`.
- Bump Vue/vite/vue-router/`@vitejs/plugin-vue` to latest; confirm `npm run build` + dev server work.

## Phase 1 — Tooling foundation (no UI changes yet)
- TypeScript: `tsconfig` (+ `allowJs`), `vue-tsc` as `typecheck` script; Vite env types.
- ESLint (flat config, `eslint-plugin-vue` essential + `@vue/eslint-config-typescript`) + Prettier; add `lint` script.
- Test stack: Vitest + `@vue/test-utils` + jsdom (unit/component), `msw` to mock the backend API, and Playwright (e2e). Scripts: `test`, `test:coverage`, `test:e2e`.
- Tailwind CSS v4 (`@tailwindcss/vite`), `shadcn-vue` init (reka-ui, `class-variance-authority`, `tailwind-merge`, `lucide-vue-next`), `@/` alias already exists.
- Pinia for shared state; `@vueuse/core` for composables.
- CI: run lint + typecheck + unit tests (with coverage) + build in the existing GitHub Actions setup; e2e as a separate job.

## Phase 1b — Characterization tests (before touching any component)
Lock in current behavior so the rewrite can be checked against it. Written against the *existing* Options-API components, then kept passing through the migration:
- Playwright e2e on the critical flows, with the API mocked (msw / `page.route`): login → home, signup, upload photo, move photo to folder (modal), create/delete folder, tag + search, gallery browse, map marker render, admin user list.
- Component tests for the logic-heavy pieces: `MediaList` (props/emits/exposed methods `updateNsId`, `updatePhotoList`), `MoveToShowFolder` (`openModal`, `moveToUpdate`), `Login`/`Signup` (validation, redirect, token storage).
- Assertions target user-visible behavior (text, roles, emitted events, API calls), **not** Naive UI class names or DOM structure, so they survive the shadcn-vue rewrite.

## Phase 2 — Design system & app shell
- Theme tokens (CSS variables) with light/dark mode via `useColorMode` (VueUse); system-preference default.
- Responsive app shell replacing `NavigationBar`/page layouts: sidebar on desktop, bottom tab bar / drawer on mobile; consistent spacing, typography, and empty/loading/error states.
- Shared building blocks used by all pages: toast (`vue-sonner`, replaces Naive `useMessage`), dialog/confirm, form fields, skeletons.
- Shared composables extracted from duplicated logic (axios client with auth/401 handling, `usePhotos`, `useFolders`) — Pinia stores where state is truly shared.
- Router cleanup: lazy-loaded routes, `meta.requiresAuth` guard, use `useRouter()` instead of importing the router instance in components.

## Phase 3 — Page-by-page migration (script setup + TS + shadcn, one pass each)
Order small → large so patterns settle before the hard files:
1. Auth: `Login`, `Signup`, `Welcome`
2. Admin: `AdminDashboard`, `AdminUserManagement`
3. Small components: `SearchPart`, `MapPart`, `TagPart`, `MoveToShowFolder`
4. Core: `GalleryPart`, `ManagePart`, `MediaList` (1140 lines; responsive grid, virtual scrolling for large libraries, lightbox viewer), `HomePart` (985 lines)
5. Delete dead code (`Template.vue`, `HomePartTest.vue`) after confirming unused.

Definition of done per file: its Phase 1b tests still pass unchanged (selectors by role/label/text), new component test added for any logic extracted to a composable/store, `lint` + `typecheck` clean.

Per-file recipe: merge the two script blocks; `data`→`ref/reactive`, `methods`→functions, `computed/watch`→composition equivalents, `mounted`→`onMounted`, `beforeDestroy`→`onBeforeUnmount`, `this.$emit/props/expose`→`defineEmits/defineProps/defineExpose` (parent→child ref calls in `MediaList`/`MoveToShowFolder` need `defineExpose`), Naive `n-*` → shadcn-vue components + Tailwind classes, `@vicons/fluent` → lucide. Keep Leaflet as is (wrap in a small `useLeafletMap` composable; `RepeatMarker.js` untouched).

## Phase 4 — Cross-platform polish
- `vite-plugin-pwa`: manifest, icons, service worker (cache app shell; runtime cache for thumbnails), install prompt.
- Mobile ergonomics: touch targets, safe-area insets, pull/scroll behavior, `<meta viewport>`; test at 375 / 768 / 1280 widths and on iOS Safari + Android Chrome.
- Accessibility pass (focus states, aria labels on icon buttons, contrast in both themes); image lazy-loading + responsive `srcset`/thumbnails.

## Phase 5 — Cleanup
- Remove `naive-ui` and `@vicons/fluent`; verify bundle size drop.
- Update README (stack, scripts); `grep` confirms no `export default {`, `defineComponent`, `this.`, `$refs` left in `src`.

## Verification
- Every phase: `npm run lint && npm run typecheck && npm run test && npm run build` green; Playwright e2e green before merging each phase.
- Unit tests for the shell, auth forms, composables, and Pinia stores; coverage reported in CI (target: no regression, high coverage on composables/stores rather than a blanket %).
- Playwright e2e also run at mobile (375px) and desktop viewports, light and dark, and in Chromium + WebKit.
- Manual smoke in `npm run dev` per page (login/signup, upload, move-to-folder, map markers, gallery, tags, search, manage, admin) in light/dark and at mobile/desktop widths; Lighthouse PWA + accessibility audit at the end.

## Risks / notes
- Biggest cost is Phase 3 (full template rewrite of 18 files from Naive UI → shadcn-vue); MediaList and HomePart dominate. Each phase is mergeable on its own, so work can pause without leaving the app broken.
- There are no existing tests, so Phase 1b is the gate: don't start Phase 3 until the characterization tests pass against the current code. Keep backend API calls unchanged.

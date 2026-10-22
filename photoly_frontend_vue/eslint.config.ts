import { globalIgnores } from 'eslint/config'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import pluginVue from 'eslint-plugin-vue'
import skipFormatting from '@vue/eslint-config-prettier/skip-formatting'

export default defineConfigWithVueTs(
  { name: 'app/files-to-lint', files: ['**/*.{ts,mts,vue}'] },
  globalIgnores([
    '**/dist/**',
    '**/coverage/**',
    'playwright-report/**',
    'test-results/**',
    'test-results-real/**',
  ]),
  // Legacy files not yet migrated.
  // Phase 3: delete each entry as its file moves to <script setup lang="ts">.
  globalIgnores([
    'src/components/GalleryPart.vue',
    'src/components/HomePart.vue',
    'src/components/HomePartTest.vue',
    'src/components/ManagePart.vue',
    'src/components/MapPart.vue',
    'src/components/TagPart.vue',
    'src/views/Login.vue',
    'src/views/Signup.vue',
    'src/views/Template.vue',
    'src/views/Welcome.vue',
  ]),
  pluginVue.configs['flat/essential'],
  vueTsConfigs.recommended,
  // shadcn-vue's generated primitives use single-word names (Label, Sheet, ...).
  {
    name: 'app/shadcn-ui',
    files: ['src/components/ui/**'],
    rules: { 'vue/multi-word-component-names': 'off' },
  },
  skipFormatting,
)

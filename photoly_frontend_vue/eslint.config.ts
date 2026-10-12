import { globalIgnores } from 'eslint/config'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import pluginVue from 'eslint-plugin-vue'
import skipFormatting from '@vue/eslint-config-prettier/skip-formatting'

export default defineConfigWithVueTs(
  { name: 'app/files-to-lint', files: ['**/*.{ts,mts,vue}'] },
  globalIgnores(['**/dist/**', '**/coverage/**', 'playwright-report/**', 'test-results/**']),
  // Vendored Leaflet plugin, plus legacy Options-API files not yet migrated.
  // Phase 3: delete each entry as its file moves to <script setup lang="ts">.
  globalIgnores([
    'src/resources/RepeatMarker.js',
    'src/components/GalleryPart.vue',
    'src/components/HomePart.vue',
    'src/components/HomePartTest.vue',
    'src/components/ManagePart.vue',
    'src/components/MapPart.vue',
    'src/components/MediaList.vue',
    'src/components/MoveToShowFolder.vue',
    'src/components/NavigationBar.vue',
    'src/components/SearchPart.vue',
    'src/components/TagPart.vue',
    'src/views/AdminDashboard.vue',
    'src/views/AdminUserManagement.vue',
    'src/views/Home.vue',
    'src/views/Login.vue',
    'src/views/Signup.vue',
    'src/views/Template.vue',
    'src/views/Welcome.vue',
  ]),
  pluginVue.configs['flat/essential'],
  vueTsConfigs.recommended,
  skipFormatting,
)

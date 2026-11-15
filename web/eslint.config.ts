import { readFileSync } from 'node:fs'

import { globalIgnores } from 'eslint/config'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import pluginVue from 'eslint-plugin-vue'
import pluginOxlint from 'eslint-plugin-oxlint'
import unocss from '@unocss/eslint-config/flat'
import skipFormatting from 'eslint-config-prettier/flat'

// unplugin-auto-import 生成的全局变量清单
const autoImport = JSON.parse(
  readFileSync(new URL('.eslintrc-auto-import.json', import.meta.url), 'utf8'),
)

export default defineConfigWithVueTs(
  {
    name: 'app/files-to-lint',
    files: ['**/*.{ts,mts,tsx,vue}'],
  },

  globalIgnores(['**/dist/**', '**/dist-ssr/**', '**/coverage/**']),

  ...pluginVue.configs['flat/essential'],
  vueTsConfigs.recommended,
  unocss,
  {
    name: 'app/auto-import-globals',
    languageOptions: { globals: autoImport.globals },
  },

  {
    name: 'app/rules',
    files: ['**/*.{ts,mts,tsx,vue}'],
    rules: {
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-unused-vars': 'off',
      '@typescript-eslint/no-unused-expressions': 'off',
      '@typescript-eslint/no-empty-function': 'off',
      '@typescript-eslint/no-non-null-assertion': 'off',
      '@typescript-eslint/no-empty-object-type': 'off',
    },
  },

  ...pluginOxlint.buildFromOxlintConfigFile('.oxlintrc.json'),

  skipFormatting,
)

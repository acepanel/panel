import AutoImport from 'unplugin-auto-import/vite'
import { NaiveUiResolver } from 'unplugin-vue-components/resolvers'
import Components from 'unplugin-vue-components/vite'

/**
 * * unplugin-icons应用，自动引入iconify图标
 * usage: https://github.com/antfu/unplugin-icons
 * 图标库: https://icones.js.org/
 */
import IconsResolver from 'unplugin-icons/resolver'
import Icons from 'unplugin-icons/vite'

export default [
  AutoImport({
    imports: [
      'vue',
      'vue-router',
      'pinia',
      '@vueuse/core',
      {
        'alova/client': [
          'actionDelegationMiddleware',
          'accessAction',
          'createClientTokenAuthentication',
          'createServerTokenAuthentication',
          'updateState',
          'useAutoRequest',
          'useCaptcha',
          'useFetcher',
          'useForm',
          'usePagination',
          'useRequest',
          'useRetriable',
          'useSQRequest',
          'useSSE',
          'useSerialRequest',
          'useSerialWatcher',
          'useWatcher'
        ]
      }
    ],
    dts: 'src/types/auto-imports.d.ts',
    eslintrc: {
      enabled: true
    },
    parser: 'acorn',
    vueTemplate: true,
    viteOptimizeDeps: true
  }),
  Components({
    dirs: ['src/components', 'src/components/system'],
    resolvers: [
      NaiveUiResolver(),
      IconsResolver()
    ],
    dts: 'src/types/components.d.ts'
  }),
  Icons({
    compiler: 'vue3',
    scale: 1,
    defaultClass: 'inline-block',
    autoInstall: true
  }),
]

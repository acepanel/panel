import type { RouteType } from '~/types/router'

const Layout = () => import('@/layout/IndexView.vue')

export default {
  name: 'toolbox',
  path: '/toolbox',
  component: Layout,
  meta: {
    title: 'Toolbox',
    icon: 'mdi:tools',
    order: 90
  },
  children: [
    {
      name: 'toolbox-system',
      path: 'system',
      component: () => import('./SystemView.vue'),
      meta: {
        title: 'System',
        role: ['admin'],
        requireAuth: true
      }
    },
    {
      name: 'toolbox-benchmark',
      path: 'benchmark',
      component: () => import('./BenchmarkView.vue'),
      meta: {
        title: 'Benchmark',
        role: ['admin'],
        requireAuth: true
      }
    },
    {
      name: 'toolbox-migration',
      path: 'migration',
      component: () => import('./MigrationView.vue'),
      meta: {
        title: '迁移',
        role: ['admin'],
        requireAuth: true
      }
    }
  ]
} as RouteType

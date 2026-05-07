import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'clickhouse',
  path: '/apps/clickhouse',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-clickhouse-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'ClickHouse',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

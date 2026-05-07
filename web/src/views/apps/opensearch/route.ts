import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'opensearch',
  path: '/apps/opensearch',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-opensearch-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'OpenSearch',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'elasticsearch',
  path: '/apps/elasticsearch',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-elasticsearch-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'ElasticSearch',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

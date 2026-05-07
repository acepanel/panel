import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'kafka',
  path: '/apps/kafka',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-kafka-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Kafka',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

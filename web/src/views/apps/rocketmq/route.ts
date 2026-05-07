import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'rocketmq',
  path: '/apps/rocketmq',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-rocketmq-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'RocketMQ',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

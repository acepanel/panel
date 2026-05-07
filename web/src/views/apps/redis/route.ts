import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'redis',
  path: '/apps/redis',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-redis-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Redis',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

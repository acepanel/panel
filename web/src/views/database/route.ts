import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'database',
  path: '/database',
  component: Layout,
  meta: {
    order: 5,
  },
  children: [
    {
      name: 'database-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Database',
        icon: 'mdi:database-outline',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

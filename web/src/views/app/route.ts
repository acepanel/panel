import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'app',
  path: '/app',
  component: Layout,
  meta: {
    order: 1,
  },
  children: [
    {
      name: 'app-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Apps',
        icon: 'mdi:apps',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

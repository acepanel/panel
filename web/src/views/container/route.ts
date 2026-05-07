import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'container',
  path: '/container',
  component: Layout,
  meta: {
    order: 2,
  },
  children: [
    {
      name: 'container-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Container',
        icon: 'mdi:layers-outline',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

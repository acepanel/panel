import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'toolbox',
  path: '/toolbox',
  component: Layout,
  meta: {
    order: 90,
  },
  children: [
    {
      name: 'toolbox-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Toolbox',
        icon: 'mdi:tools',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

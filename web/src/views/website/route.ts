import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'website',
  path: '/website',
  component: Layout,
  meta: {
    order: 3,
  },
  children: [
    {
      name: 'website-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Website',
        icon: 'mdi:web',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

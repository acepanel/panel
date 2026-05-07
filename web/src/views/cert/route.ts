import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'cert',
  path: '/cert',
  component: Layout,
  meta: {
    order: 10,
  },
  children: [
    {
      name: 'cert-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Certificate',
        icon: 'mdi:certificate-outline',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

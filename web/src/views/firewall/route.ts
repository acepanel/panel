import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'firewall',
  path: '/firewall',
  component: Layout,
  meta: {
    order: 40,
  },
  children: [
    {
      name: 'firewall-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Firewall',
        icon: 'mdi:firewall',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

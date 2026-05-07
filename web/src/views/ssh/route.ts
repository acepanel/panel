import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'ssh',
  path: '/ssh',
  component: Layout,
  meta: {
    order: 70,
  },
  children: [
    {
      name: 'ssh-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Terminal',
        icon: 'mdi:console',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

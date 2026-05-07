import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'backup',
  path: '/backup',
  component: Layout,
  meta: {
    order: 60,
  },
  children: [
    {
      name: 'backup-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Backup',
        icon: 'mdi:backup-outline',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

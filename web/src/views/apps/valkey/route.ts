import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'valkey',
  path: '/apps/valkey',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-valkey-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Valkey',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

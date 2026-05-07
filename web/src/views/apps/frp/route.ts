import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'frp',
  path: '/apps/frp',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-frp-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Frp Manager',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

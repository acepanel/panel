import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'prometheus',
  path: '/apps/prometheus',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-prometheus-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Prometheus',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

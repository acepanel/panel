import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'percona',
  path: '/apps/percona',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-percona-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Percona',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'mariadb',
  path: '/apps/mariadb',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-mariadb-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'MariaDB',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

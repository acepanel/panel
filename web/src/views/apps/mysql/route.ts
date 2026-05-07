import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'mysql',
  path: '/apps/mysql',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-mysql-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'MySQL',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

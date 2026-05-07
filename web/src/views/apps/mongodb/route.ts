import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'mongodb',
  path: '/apps/mongodb',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-mongodb-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'MongoDB',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'apache',
  path: '/apps/apache',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-apache-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Apache',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

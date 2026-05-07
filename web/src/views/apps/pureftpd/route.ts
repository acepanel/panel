import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'pureftpd',
  path: '/apps/pureftpd',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-pureftpd-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Pure-FTPd',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

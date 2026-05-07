import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'nginx',
  path: '/apps/nginx',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-nginx-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Nginx',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

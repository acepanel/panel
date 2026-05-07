import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'openresty',
  path: '/apps/openresty',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-openresty-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'OpenResty',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

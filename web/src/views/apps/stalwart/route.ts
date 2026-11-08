import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'stalwart',
  path: '/apps/stalwart',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-stalwart-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Stalwart',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

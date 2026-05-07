import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'codeserver',
  path: '/apps/codeserver',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-codeserver-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Code Server',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

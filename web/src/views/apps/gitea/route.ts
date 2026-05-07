import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'gitea',
  path: '/apps/gitea',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-gitea-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Gitea',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

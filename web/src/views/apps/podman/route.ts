import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'podman',
  path: '/apps/podman',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-podman-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Podman',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

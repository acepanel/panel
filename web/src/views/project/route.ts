import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'project',
  path: '/project',
  component: Layout,
  meta: {
    order: 4,
  },
  children: [
    {
      name: 'project-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Project',
        icon: 'mdi:folder-multiple-outline',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'file',
  path: '/file',
  component: Layout,
  meta: {
    order: 50,
  },
  children: [
    {
      name: 'file-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Files',
        icon: 'mdi:folder-open-outline',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

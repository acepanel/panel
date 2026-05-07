import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'home',
  path: '/',
  component: Layout,
  meta: {
    order: 0,
  },
  children: [
    {
      name: 'home-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Home',
        icon: 'mdi:house-outline',
        role: ['admin'],
        requireAuth: true,
      },
    },
    {
      name: 'home-update',
      path: 'update',
      isHidden: true,
      component: () => import('./UpdateView.vue'),
      meta: {
        title: 'Update',
        icon: 'mdi:archive-arrow-up-outline',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

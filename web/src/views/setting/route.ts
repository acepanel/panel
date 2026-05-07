import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'setting',
  path: '/setting',
  component: Layout,
  meta: {
    order: 999,
  },
  children: [
    {
      name: 'setting-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Setting',
        icon: 'mdi:settings-outline',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

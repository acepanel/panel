import type { RouteType } from '~/types/router'

const Layout = () => import('@/layout/IndexView.vue')

export default {
  name: 'log',
  path: '/log',
  component: Layout,
  meta: {
    order: 35
  },
  children: [
    {
      name: 'log-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Logs',
        icon: 'mdi:file-document-outline',
        role: ['admin'],
        requireAuth: true
      }
    }
  ]
} as RouteType

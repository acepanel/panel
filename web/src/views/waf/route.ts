import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'waf',
  path: '/waf',
  component: Layout,
  meta: {
    order: 41,
  },
  children: [
    {
      name: 'waf-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'WAF',
        icon: 'mdi:shield-bug',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

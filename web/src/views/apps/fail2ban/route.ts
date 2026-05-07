import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'fail2ban',
  path: '/apps/fail2ban',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-fail2ban-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Fail2ban Manager',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

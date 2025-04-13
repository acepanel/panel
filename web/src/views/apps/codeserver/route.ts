import type { RouteType } from '~/types/router'

const Layout = () => import('@/layout/IndexView.vue')

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
        icon: 'simple-icons:coder',
        role: ['admin'],
        requireAuth: true
      }
    }
  ]
} as RouteType

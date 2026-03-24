import type { RouteType } from '~/types/router'

const Layout = () => import('@/layout/IndexView.vue')

export default {
  name: 'grafana',
  path: '/apps/grafana',
  component: Layout,
  isHidden: true,
  children: [
    {
      name: 'apps-grafana-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Grafana',
        role: ['admin'],
        requireAuth: true
      }
    }
  ]
} as RouteType

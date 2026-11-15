import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')
const RuntimeView = () => import('./RuntimeView.vue')

export default {
  name: 'environment',
  path: '/environment',
  isHidden: true,
  component: Layout,
  children: [
    {
      name: 'environment-go',
      path: 'go/:slug',
      isHidden: true,
      component: RuntimeView,
      meta: {
        title: 'Go',
        icon: 'mdi:language-go',
        role: ['admin'],
        requireAuth: true,
        runtime: 'go',
      },
    },
    {
      name: 'environment-java',
      path: 'java/:slug',
      isHidden: true,
      component: RuntimeView,
      meta: {
        title: 'Java',
        icon: 'mdi:language-java',
        role: ['admin'],
        requireAuth: true,
        runtime: 'java',
      },
    },
    {
      name: 'environment-nodejs',
      path: 'nodejs/:slug',
      isHidden: true,
      component: RuntimeView,
      meta: {
        title: 'Node.js',
        icon: 'mdi:nodejs',
        role: ['admin'],
        requireAuth: true,
        runtime: 'nodejs',
      },
    },
    {
      name: 'environment-php',
      path: 'php/:slug',
      isHidden: true,
      component: () => import('./PHPView.vue'),
      meta: {
        title: 'PHP',
        icon: 'mdi:language-php',
        role: ['admin'],
        requireAuth: true,
      },
    },
    {
      name: 'environment-python',
      path: 'python/:slug',
      isHidden: true,
      component: RuntimeView,
      meta: {
        title: 'Python',
        icon: 'mdi:language-python',
        role: ['admin'],
        requireAuth: true,
        runtime: 'python',
      },
    },
    {
      name: 'environment-dotnet',
      path: 'dotnet/:slug',
      isHidden: true,
      component: RuntimeView,
      meta: {
        title: '.NET',
        icon: 'mdi:dot-net',
        role: ['admin'],
        requireAuth: true,
        runtime: 'dotnet',
      },
    },
  ],
} as RouteType

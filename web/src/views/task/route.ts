import type { RouteType } from '@/types/router'

const Layout = () => import('@/layouts/IndexView.vue')

export default {
  name: 'task',
  path: '/task',
  component: Layout,
  meta: {
    order: 80,
  },
  children: [
    {
      name: 'task-index',
      path: '',
      component: () => import('./IndexView.vue'),
      meta: {
        title: 'Task',
        icon: 'mdi:timetable',
        role: ['admin'],
        requireAuth: true,
      },
    },
  ],
} as RouteType

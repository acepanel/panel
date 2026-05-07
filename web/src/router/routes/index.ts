import type { RouteModule, RoutesType, RouteType } from '@/types/router'
import { $gettext } from '@/utils/gettext'

export const basicRoutes: RoutesType = [
  {
    name: '404',
    path: '/404',
    component: () => import('@/views/error-page/NotFound.vue'),
    isHidden: true,
  },

  {
    name: 'Login',
    path: '/login',
    component: () => import('@/views/login/IndexView.vue'),
    isHidden: true,
    meta: {
      title: $gettext('Login'),
    },
  },
]

export const NOT_FOUND_ROUTE: RouteType = {
  name: 'NotFound',
  path: '/:pathMatch(.*)*',
  redirect: '/404',
  isHidden: true,
}

export const EMPTY_ROUTE: RouteType = {
  name: 'Empty',
  path: '/:pathMatch(.*)*',
  component: () => {},
}

const modules = import.meta.glob('@/views/**/route.ts', {
  eager: true,
}) as RouteModule
const asyncRoutes: RoutesType = []
Object.keys(modules).forEach((key) => {
  const route = modules[key]?.default
  if (route) asyncRoutes.push(route)
})

export { asyncRoutes }

import type { RoutesType, RouteType } from '@/types/router'

export const basicRoutes: RoutesType = [
  {
    name: '404',
    path: '/404',
    component: () => import('@/views/error-page/NotFound.vue'),
    isHidden: true,
  },

  // 防火墙与防篡改已并入安全菜单,重定向兼容旧路径与持久化的标签页
  {
    name: 'firewall-redirect',
    path: '/firewall',
    redirect: '/safe',
    isHidden: true,
  },
  {
    name: 'tamper-redirect',
    path: '/tamper',
    redirect: '/safe',
    isHidden: true,
  },
  {
    name: 'safe-firewall-redirect',
    path: '/safe/firewall',
    redirect: '/safe',
    isHidden: true,
  },
  {
    name: 'safe-tamper-redirect',
    path: '/safe/tamper',
    redirect: '/safe',
    isHidden: true,
  },

  {
    name: 'Login',
    path: '/login',
    component: () => import('@/views/login/IndexView.vue'),
    isHidden: true,
    meta: {
      title: 'Login',
    },
  },
]

export const NOT_FOUND_ROUTE: RouteType = {
  name: 'NotFound',
  path: '/:pathMatch(.*)*',
  redirect: '/404',
  isHidden: true,
}

// 各模块 route.ts 的默认导出可以是单条路由，也可以是路由数组
const modules = import.meta.glob<{ default?: RouteType | RoutesType }>('@/views/**/route.ts', {
  eager: true,
})

export const asyncRoutes: RoutesType = Object.values(modules).flatMap((m) => m.default ?? [])

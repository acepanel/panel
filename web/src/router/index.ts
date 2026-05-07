import type { App } from 'vue'
import { createRouter, createWebHashHistory, createWebHistory } from 'vue-router'

import { usePermissionStore } from '@/stores'
import type { RoutesType, RouteType } from '@/types/router'

import { setupRouterGuard } from './guard'
import { basicRoutes, EMPTY_ROUTE, NOT_FOUND_ROUTE } from './routes'

const isHash = import.meta.env.VITE_USE_HASH === 'true'
export const router = createRouter({
  history: isHash
    ? createWebHashHistory(import.meta.env.VITE_PUBLIC_PATH || '/')
    : createWebHistory(import.meta.env.VITE_PUBLIC_PATH || '/'),
  routes: basicRoutes,
  scrollBehavior: () => ({ left: 0, top: 0 }),
})

export async function setupRouter(app: App) {
  await addDynamicRoutes()
  setupRouterGuard(router)
  app.use(router)
}

export async function addDynamicRoutes() {
  try {
    const permissionStore = usePermissionStore()
    const accessRoutes = permissionStore.generateRoutes(['admin'])
    accessRoutes.forEach((route: RouteType) => {
      if (!router.hasRoute(route.name)) router.addRoute(route)
    })
    if (router.hasRoute(EMPTY_ROUTE.name)) router.removeRoute(EMPTY_ROUTE.name)
    router.addRoute(NOT_FOUND_ROUTE)
  } catch (error) {
    console.error(error)
  }
}

export async function resetRouter() {
  const basicRouteNames = getRouteNames(basicRoutes)
  router.getRoutes().forEach((route) => {
    const name = route.name as string
    if (!basicRouteNames.includes(name)) router.removeRoute(name)
  })
}

export function getRouteNames(routes: RoutesType) {
  return routes.map((route) => getRouteName(route)).flat(1)
}

function getRouteName(route: RouteType) {
  const names = [route.name]
  if (route.children && route.children.length)
    names.push(...route.children.map((item) => getRouteName(item as RouteType)).flat(1))

  return names
}

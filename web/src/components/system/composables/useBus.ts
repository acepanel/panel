import type { Handler } from 'mitt'

/**
 * 订阅全局事件总线，所在作用域销毁时自动解绑
 */
export function useBus<K extends keyof BusEvents>(event: K, handler: Handler<BusEvents[K]>) {
  window.$bus.on(event, handler)
  onScopeDispose(() => window.$bus.off(event, handler))
}

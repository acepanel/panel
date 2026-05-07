import { useThemeStore } from '@/stores'

export type Breakpoint = 'mobile' | 'tablet' | 'desktop' | 'wide'

/**
 * 响应式断点 hook
 * - mobile  : <  768
 * - tablet  : 768 - 1023
 * - desktop : 1024 - 1439
 * - wide    : >= 1440
 */
export function useResponsive() {
  const themeStore = useThemeStore()
  const width = ref(typeof window !== 'undefined' ? window.innerWidth : 1280)

  const handler = () => {
    width.value = window.innerWidth
  }

  onMounted(() => {
    handler()
    window.addEventListener('resize', handler)
  })

  onBeforeUnmount(() => {
    window.removeEventListener('resize', handler)
  })

  const breakpoint = computed<Breakpoint>(() => {
    if (width.value < 768) return 'mobile'
    if (width.value < 1024) return 'tablet'
    if (width.value < 1440) return 'desktop'
    return 'wide'
  })

  const isMobile = computed(() => breakpoint.value === 'mobile' || themeStore.isMobile)
  const isTablet = computed(() => breakpoint.value === 'tablet')
  const isDesktop = computed(() => breakpoint.value === 'desktop' || breakpoint.value === 'wide')

  return { width, breakpoint, isMobile, isTablet, isDesktop }
}

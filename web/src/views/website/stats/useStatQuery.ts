import { useTablePagination } from '@/components/system/composables/useTablePagination'

interface StatQueryParams {
  start: string
  end: string
  sites: string
  page: number
  limit: number
}

/**
 * 统计标签页取数：时间范围、站点筛选或刷新变化时回到第一页重新请求
 */
export function useStatQuery(fetch: (params: StatQueryParams) => any) {
  const ctx = inject<any>('statContext')!

  const loading = ref(false)
  const data = shallowRef<any>(null)
  const items = computed<any[]>(() => data.value?.items || [])
  const total = computed<number>(() => data.value?.total || 0)
  const page = ref(1)
  const limit = ref(50)
  // 连续切换筛选时只认最后一次请求，先发后到的结果不能覆盖数据或提前结束加载态
  let seq = 0

  const load = () => {
    const current = ++seq
    loading.value = true
    useRequest(
      fetch({
        start: ctx.dateRange.value.start,
        end: ctx.dateRange.value.end,
        sites: ctx.sitesParam.value,
        page: page.value,
        limit: limit.value,
      }),
    )
      .onSuccess(({ data: res }: any) => {
        if (current === seq) data.value = res
      })
      .onComplete(() => {
        if (current === seq) loading.value = false
      })
  }

  const reload = () => {
    page.value = 1
    load()
  }

  const base = useTablePagination(page, limit, total, [20, 50, 100])
  const pagination = computed(() => ({
    ...base.value,
    onUpdatePage: (p: number) => {
      page.value = p
      load()
    },
    onUpdatePageSize: (s: number) => {
      limit.value = s
      reload()
    },
  }))

  watch([() => ctx.dateRange.value, () => ctx.sitesParam.value, () => ctx.refreshKey.value], reload)
  onMounted(load)

  return { loading, data, items, total, pagination, reload }
}

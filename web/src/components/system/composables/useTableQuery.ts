import type { FetchFn } from '../types'

interface UseTableQueryOptions<T = any> {
  fetch: FetchFn<T>
  initialPageSize?: number
  initialQuery?: Record<string, any>
  immediate?: boolean
}

/**
 * 列表分页查询 composable
 */
export function useTableQuery<T = any>(opts: UseTableQueryOptions<T>) {
  const page = ref(1)
  const pageSize = ref(opts.initialPageSize ?? 20)
  const total = ref(0)
  const items = ref<T[]>([]) as Ref<T[]>
  const loading = ref(false)
  const query = ref<Record<string, any>>({ ...opts.initialQuery })

  const load = async () => {
    loading.value = true
    try {
      const result = await opts.fetch({
        page: page.value,
        pageSize: pageSize.value,
        query: { ...query.value },
      })
      items.value = result.items ?? []
      total.value = result.total ?? 0
    } finally {
      loading.value = false
    }
  }

  const refresh = () => load()

  const setPage = (p: number) => {
    page.value = p
    load()
  }

  const setPageSize = (size: number) => {
    pageSize.value = size
    page.value = 1
    load()
  }

  const setQuery = (q: Record<string, any>) => {
    query.value = { ...q }
    page.value = 1
    load()
  }

  const reset = () => {
    page.value = 1
    query.value = { ...opts.initialQuery }
    load()
  }

  if (opts.immediate !== false) {
    onMounted(() => load())
  }

  return {
    page,
    pageSize,
    total,
    items,
    loading,
    query,
    load,
    refresh,
    setPage,
    setPageSize,
    setQuery,
    reset,
  }
}

import type { AlovaGenerics, Method } from 'alova'
import type { PaginationHookConfig } from 'alova/client'
import type { VueHookExportType } from 'alova/vue'

import { useTablePagination } from './useTablePagination'

type AG = AlovaGenerics<any, any, any, any, any, any, any, VueHookExportType<unknown>>

interface PagedTableOptions extends PaginationHookConfig<AG, any[]> {
  pageSizes?: number[]
}

/**
 * 分页列表：后端统一返回 { total, items }，默认每页 20 条
 */
export function usePagedTable(
  fetcher: (page: number, pageSize: number) => Method<AG>,
  { pageSizes, ...config }: PagedTableOptions = {},
) {
  const { loading, data, page, pageSize, total, refresh, reload } = usePagination(fetcher, {
    initialData: { total: 0, items: [] },
    initialPageSize: 20,
    total: (res) => res.total,
    data: (res) => res.items,
    ...config,
  })
  const pagination = useTablePagination(page, pageSize, total, pageSizes)

  return { loading, data, page, pageSize, total, refresh, reload, pagination }
}

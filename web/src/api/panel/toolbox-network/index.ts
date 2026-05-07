import { http } from '@/utils'

export interface NetworkListParams {
  page: number
  limit: number
  sort?: string
  order?: string
  state?: string // 逗号分隔
  pid?: string
  process?: string
  port?: string
}

export default {
  list: (params: NetworkListParams) => http.Get('/toolbox_network/list', { params }),
}

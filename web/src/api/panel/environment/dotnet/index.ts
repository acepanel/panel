import { http } from '@/utils'

export default {
  // 设为 CLI 版本
  setCli: (slug: string): any => http.Post(`/environment/dotnet/${slug}/set_cli`),
}

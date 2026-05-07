import { http } from '@/utils'

export default {
  // 获取环境变量
  env: (): any => http.Get('/apps/minio/env'),
  // 保存环境变量
  saveEnv: (env: string): any => http.Post('/apps/minio/env', { env }),
}

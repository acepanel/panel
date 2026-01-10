import { http } from '@/utils'

export default {
  // 获取容器列表
  containerList: (page: number, limit: number): any =>
    http.Get('/container/container', { params: { page, limit } }),
  // 添加容器
  containerCreate: (config: any): any => http.Post('/container/container', config),
  // 删除容器
  containerRemove: (id: string): any => http.Delete(`/container/container/${id}`),
  // 启动容器
  containerStart: (id: string): any => http.Post(`/container/container/${id}/start`),
  // 停止容器
  containerStop: (id: string): any => http.Post(`/container/container/${id}/stop`),
  // 重启容器
  containerRestart: (id: string): any => http.Post(`/container/container/${id}/restart`),
  // 暂停容器
  containerPause: (id: string): any => http.Post(`/container/container/${id}/pause`),
  // 恢复容器
  containerUnpause: (id: string): any => http.Post(`/container/container/${id}/unpause`),
  // 杀死容器
  containerKill: (id: string): any => http.Post(`/container/container/${id}/kill`),
  // 重命名容器
  containerRename: (id: string, name: string): any =>
    http.Post(`/container/container/${id}/rename`, { name }),
  // 获取容器日志
  containerLogs: (id: string): any => http.Get(`/container/container/${id}/logs`),
  // 清理容器
  containerPrune: (): any => http.Post(`/container/container/prune`),
  // 获取编排列表
  composeList: (page: number, limit: number): any =>
    http.Get('/container/compose', { params: { page, limit } }),
  // 获取编排
  composeGet: (name: string): any => http.Get(`/container/compose/${name}`),
  // 创建编排
  composeCreate: (config: any): any => http.Post('/container/compose', config),
  // 更新编排
  composeUpdate: (name: string, config: any): any => http.Put(`/container/compose/${name}`, config),
  // 删除编排
  composeRemove: (name: string): any => http.Delete(`/container/compose/${name}`),
  // 启动编排
  composeUp: (name: string, force: boolean): any =>
    http.Post(`/container/compose/${name}/up`, { force }),
  // 停止编排
  composeDown: (name: string): any => http.Post(`/container/compose/${name}/down`),
  // 获取网络列表
  networkList: (page: number, limit: number): any =>
    http.Get(`/container/network`, { params: { page, limit } }),
  // 创建网络
  networkCreate: (config: any): any => http.Post(`/container/network`, config),
  // 删除网络
  networkRemove: (id: string): any => http.Delete(`/container/network/${id}`),
  // 清理网络
  networkPrune: (): any => http.Post(`/container/network/prune`),
  // 获取镜像列表
  imageList: (page: number, limit: number): any =>
    http.Get(`/container/image`, { params: { page, limit } }),
  // 检查镜像是否存在
  imageExist: (name: string): any => http.Get(`/container/image/exist`, { params: { name } }),
  // 拉取镜像
  imagePull: (config: any): any => http.Post(`/container/image`, config),
  // 删除镜像
  imageRemove: (id: string): any => http.Delete(`/container/image/${id}`),
  // 清理镜像
  imagePrune: (): any => http.Post(`/container/image/prune`),
  // 获取卷列表
  volumeList: (page: number, limit: number): any =>
    http.Get(`/container/volume`, { params: { page, limit } }),
  // 创建卷
  volumeCreate: (config: any): any => http.Post(`/container/volume`, config),
  // 删除卷
  volumeRemove: (id: string): any => http.Delete(`/container/volume/${id}`),
  // 清理卷
  volumePrune: (): any => http.Post(`/container/volume/prune`)
}

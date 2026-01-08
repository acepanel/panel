import { http } from '@/utils'

export default {
  // 创建文件/文件夹
  create: (path: string, dir: boolean): any => http.Post('/file/create', { path, dir }),
  // 获取文件内容
  content: (path: string): any => http.Get('/file/content', { params: { path } }),
  // 保存文件
  save: (path: string, content: string): any => http.Post('/file/save', { path, content }),
  // 删除文件
  delete: (path: string): any => http.Post('/file/delete', { path }),
  // 上传文件
  upload: (formData: FormData): any => http.Post('/file/upload', formData),
  // 检查文件是否存在
  exist: (paths: string[]): any => http.Post('/file/exist', paths),
  // 移动文件
  move: (paths: any[]): any => http.Post('/file/move', paths),
  // 复制文件
  copy: (paths: any[]): any => http.Post('/file/copy', paths),
  // 远程下载
  remoteDownload: (path: string, url: string): any =>
    http.Post('/file/remote_download', { path, url }),
  // 获取文件信息
  info: (path: string): any => http.Get('/file/info', { params: { path } }),
  // 获取目录/文件大小
  size: (path: string): any => http.Get('/file/size', { params: { path } }),
  // 修改文件权限
  permission: (path: string, mode: string, owner: string, group: string): any =>
    http.Post('/file/permission', { path, mode, owner, group }),
  // 压缩文件
  compress: (dir: string, paths: string[], file: string): any =>
    http.Post('/file/compress', { dir, paths, file }),
  // 解压文件
  unCompress: (file: string, path: string): any => http.Post('/file/un_compress', { file, path }),
  // 获取文件列表
  list: (
    path: string,
    keyword: string,
    sub: boolean,
    sort: string,
    page: number,
    limit: number
  ): any => http.Get('/file/list', { params: { path, keyword, sub, sort, page, limit } })
}

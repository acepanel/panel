export interface Marked {
  name: string
  source: string
  force: boolean
}

// 文件信息接口，用于权限编辑和属性显示
export interface FileInfo {
  name: string
  full: string
  size: string
  mode_str: string
  mode: string
  owner: string
  group: string
  uid: number
  gid: number
  hidden: boolean
  symlink: boolean
  link: string
  dir: boolean
  modify: string
  immutable: boolean
}

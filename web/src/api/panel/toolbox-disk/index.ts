import { http } from '@/utils'

export default {
  // 获取磁盘列表
  list: (): any => http.Get('/toolbox_disk/list'),
  // 获取分区列表
  partitions: (device: string): any => http.Post('/toolbox_disk/partitions', { device }),
  // 挂载分区
  mount: (device: string, path: string): any => http.Post('/toolbox_disk/mount', { device, path }),
  // 卸载分区
  umount: (path: string): any => http.Post('/toolbox_disk/umount', { path }),
  // 格式化分区
  format: (device: string, fs_type: string): any =>
    http.Post('/toolbox_disk/format', { device, fs_type }),
  // 获取LVM信息
  lvmInfo: (): any => http.Get('/toolbox_disk/lvm'),
  // 创建物理卷
  createPV: (device: string): any => http.Post('/toolbox_disk/lvm/pv', { device }),
  // 删除物理卷
  removePV: (device: string): any => http.Delete('/toolbox_disk/lvm/pv', { device }),
  // 创建卷组
  createVG: (name: string, devices: string[]): any =>
    http.Post('/toolbox_disk/lvm/vg', { name, devices }),
  // 删除卷组
  removeVG: (name: string): any => http.Delete('/toolbox_disk/lvm/vg', { name }),
  // 创建逻辑卷
  createLV: (name: string, vg_name: string, size: number): any =>
    http.Post('/toolbox_disk/lvm/lv', { name, vg_name, size }),
  // 删除逻辑卷
  removeLV: (path: string): any => http.Delete('/toolbox_disk/lvm/lv', { path }),
  // 扩容逻辑卷
  extendLV: (path: string, size: number, resize: boolean): any =>
    http.Post('/toolbox_disk/lvm/lv/extend', { path, size, resize })
}

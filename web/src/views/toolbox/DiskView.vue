<script setup lang="ts">
defineOptions({
  name: 'toolbox-disk'
})

import { useRequest } from 'alova/client'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NProgress, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import disk from '@/api/panel/toolbox-disk'
import { formatBytes } from '@/utils'

// lsblk JSON 输出的数据结构
interface BlockDevice {
  name: string
  size: number
  type: string
  mountpoint: string | null
  fstype: string | null
  uuid: string | null
  label: string | null
  model: string | null
  children?: BlockDevice[]
}

// 分区展示数据
interface PartitionData {
  name: string
  size: number
  used: number
  available: number
  usagePercent: number
  mountpoint: string | null
  fstype: string | null
  isSystemDisk: boolean
}

// 磁盘展示数据
interface DiskData {
  name: string
  size: number
  type: string
  model: string | null
  isSystemDisk: boolean
  partitions: PartitionData[]
}

const { $gettext, $pgettext } = useGettext()
const currentTab = ref('disk')
const diskList = ref<DiskData[]>([])
const lvmInfo = ref<any>({ pvs: [], vgs: [], lvs: [] })

// 磁盘管理
const selectedDevice = ref('')
const mountPath = ref('')
const mountWriteFstab = ref(false)
const mountOption = ref('')
const formatDevice = ref('')
const formatFsType = ref('ext4')
const initDevice = ref('')
const initFsType = ref('ext4')
const fsTypeOptions = [
  { label: 'ext4', value: 'ext4' },
  { label: 'ext3', value: 'ext3' },
  { label: 'xfs', value: 'xfs' },
  { label: 'btrfs', value: 'btrfs' }
]

// fstab 管理
interface FstabEntry {
  device: string
  mount_point: string
  fs_type: string
  options: string
  dump: string
  pass: string
}
const fstabList = ref<FstabEntry[]>([])

// LVM管理
const pvDevice = ref('')
const vgName = ref('')
const vgDevices = ref<string[]>([])
const lvName = ref('')
const lvVgName = ref('')
const lvSize = ref(1)
const extendLvPath = ref('')
const extendSize = ref(1)
const extendResize = ref(true)

// df 数据类型
interface DfInfo {
  size: string
  used: string
  avail: string
  percent: string
}

// 加载磁盘列表
const loadDiskList = () => {
  useRequest(disk.list()).onSuccess(({ data }) => {
    try {
      const devices: BlockDevice[] = data.disks || []
      const dfData: Record<string, DfInfo> = data.df || {}
      diskList.value = parseDiskData(devices, dfData)
    } catch (e) {
      diskList.value = []
      window.$message.error($gettext('Failed to parse disk data, please refresh and try again'))
    }
  })
}

// 解析磁盘数据
const parseDiskData = (devices: BlockDevice[], dfData: Record<string, DfInfo>): DiskData[] => {
  const disks: DiskData[] = []

  for (const device of devices) {
    // 只处理磁盘类型
    if (device.type !== 'disk') continue

    const partitions: PartitionData[] = []
    let isSystemDisk = false

    // 先遍历一遍判断是否为系统盘
    if (device.children) {
      for (const child of device.children) {
        if (child.type === 'part' && child.mountpoint === '/') {
          isSystemDisk = true
          break
        }
      }
    }

    // 处理分区
    if (device.children) {
      for (const child of device.children) {
        if (child.type === 'part') {
          // 获取 df 数据
          const mountpoint = child.mountpoint
          const dfInfo = mountpoint ? dfData[mountpoint] : null

          partitions.push({
            name: child.name,
            size: child.size,
            used: dfInfo ? parseInt(dfInfo.used) : 0,
            available: dfInfo ? parseInt(dfInfo.avail) : 0,
            usagePercent: dfInfo ? parseInt(dfInfo.percent) : 0,
            mountpoint: child.mountpoint,
            fstype: child.fstype,
            isSystemDisk
          })
        }
      }
    }

    disks.push({
      name: device.name,
      size: device.size,
      type: device.type,
      model: device.model,
      isSystemDisk,
      partitions
    })
  }

  return disks
}

// 获取磁盘类型标签
const getDiskTypeLabel = (model: string | null): string => {
  if (!model) return $gettext('Unknown')
  const modelLower = model.toLowerCase()
  if (modelLower.includes('ssd') || modelLower.includes('nvme')) {
    return 'SSD'
  }
  return model.toUpperCase()
}

// 未挂载的分区选项（用于挂载和格式化）
const unmountedPartitionOptions = computed(() => {
  const options: { label: string; value: string }[] = []
  for (const disk of diskList.value) {
    for (const part of disk.partitions) {
      if (!part.mountpoint) {
        options.push({
          label: `${part.name} (${formatBytes(part.size)})`,
          value: part.name
        })
      }
    }
  }
  return options
})

// 非系统盘的磁盘选项（用于初始化）
const nonSystemDiskOptions = computed(() => {
  return diskList.value
    .filter((disk) => !disk.isSystemDisk)
    .map((disk) => ({
      label: `${disk.name} (${formatBytes(disk.size)})`,
      value: disk.name
    }))
})

// 可用于创建 PV 的设备选项（未加入 VG 的分区或磁盘）
const availablePVDeviceOptions = computed(() => {
  const options: { label: string; value: string }[] = []
  // 获取已有的 PV 设备列表
  const existingPVs = new Set(lvmInfo.value.pvs?.map((pv: any) => pv.field_0) || [])

  for (const disk of diskList.value) {
    // 跳过系统盘
    if (disk.isSystemDisk) continue

    // 如果磁盘没有分区，可以直接作为 PV
    if (disk.partitions.length === 0) {
      const devPath = `/dev/${disk.name}`
      if (!existingPVs.has(devPath)) {
        options.push({
          label: `${disk.name} (${formatBytes(disk.size)})`,
          value: disk.name
        })
      }
    }

    // 添加未挂载且未作为 PV 的分区
    for (const part of disk.partitions) {
      const devPath = `/dev/${part.name}`
      if (!part.mountpoint && !existingPVs.has(devPath)) {
        options.push({
          label: `${part.name} (${formatBytes(part.size)})`,
          value: part.name
        })
      }
    }
  }
  return options
})

// 可用的 PV 选项（用于创建 VG）
const availablePVOptions = computed(() => {
  return (lvmInfo.value.pvs || [])
    .filter((pv: any) => !pv.field_1) // 只显示未加入 VG 的 PV（field_1 是 VG 名）
    .map((pv: any) => ({
      label: `${pv.field_0} (${pv.field_2})`,
      value: pv.field_0
    }))
})

// VG 选项（用于创建 LV）
const vgOptions = computed(() => {
  return (lvmInfo.value.vgs || []).map((vg: any) => ({
    label: `${vg.field_0} (${$gettext('Free')}: ${vg.field_4})`,
    value: vg.field_0
  }))
})

// LV 选项（用于扩展 LV）
const lvOptions = computed(() => {
  return (lvmInfo.value.lvs || []).map((lv: any) => ({
    label: `${lv.field_0} (${lv.field_2}) - ${lv.field_3}`,
    value: lv.field_3 // 使用 LV 路径作为值
  }))
})

// 分区表格列定义
const partitionColumns = computed<DataTableColumns<PartitionData>>(() => [
  {
    title: $gettext('Partition Name'),
    key: 'name',
    width: 200
  },
  {
    title: $gettext('Size'),
    key: 'size',
    width: 120,
    render(row) {
      return formatBytes(row.size)
    }
  },
  {
    title: $gettext('Used'),
    key: 'used',
    width: 120,
    render(row) {
      if (!row.mountpoint) return '-'
      return formatBytes(row.used)
    }
  },
  {
    title: $gettext('Available'),
    key: 'available',
    width: 120,
    render(row) {
      if (!row.mountpoint) return '-'
      return formatBytes(row.available)
    }
  },
  {
    title: $gettext('Usage'),
    key: 'usagePercent',
    width: 160,
    render(row) {
      if (!row.mountpoint) {
        return h(
          NTag,
          { type: 'warning', size: 'small' },
          { default: () => $gettext('Not Mounted') }
        )
      }
      const percent = row.usagePercent
      const status = percent > 90 ? 'error' : percent > 70 ? 'warning' : 'success'
      return h(NProgress, {
        type: 'line',
        percentage: percent,
        status,
        indicatorPlacement: 'inside',
        style: { width: '120px' }
      })
    }
  },
  {
    title: $gettext('Mount Point'),
    key: 'mountpoint',
    width: 200,
    render(row) {
      return row.mountpoint || '-'
    }
  },
  {
    title: $gettext('Filesystem'),
    key: 'fstype',
    width: 100,
    render(row) {
      return row.fstype || '-'
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 120,
    render(row) {
      if (row.mountpoint) {
        return h(
          NButton,
          {
            size: 'small',
            type: 'warning',
            secondary: true,
            disabled: row.isSystemDisk,
            onClick: () => handleUmount(row.mountpoint!)
          },
          { default: () => $gettext('Unmount') }
        )
      }
      return null
    }
  }
])

// 加载LVM信息
const loadLVMInfo = () => {
  useRequest(disk.lvmInfo()).onSuccess(({ data }) => {
    lvmInfo.value = data
  })
}

// 加载fstab列表
const loadFstabList = () => {
  useRequest(disk.fstabList()).onSuccess(({ data }) => {
    fstabList.value = data || []
  })
}

onMounted(() => {
  loadDiskList()
  loadLVMInfo()
  loadFstabList()
})

// 挂载分区
const handleMount = () => {
  if (!selectedDevice.value || !mountPath.value) {
    window.$message.error($gettext('Please fill in all fields'))
    return
  }

  const confirmContent = mountWriteFstab.value
    ? $gettext(
        'Are you sure you want to mount %{ device } to %{ path } and write to fstab for auto-mount on boot?',
        {
          device: selectedDevice.value,
          path: mountPath.value
        }
      )
    : $gettext('Are you sure you want to mount %{ device } to %{ path }?', {
        device: selectedDevice.value,
        path: mountPath.value
      })

  window.$dialog.warning({
    title: $gettext('Confirm'),
    content: confirmContent,
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(
        disk.mount(selectedDevice.value, mountPath.value, mountWriteFstab.value, mountOption.value)
      ).onSuccess(() => {
        window.$message.success($gettext('Mounted successfully'))
        loadDiskList()
        if (mountWriteFstab.value) {
          loadFstabList()
        }
        selectedDevice.value = ''
        mountPath.value = ''
        mountWriteFstab.value = false
        mountOption.value = ''
      })
    }
  })
}

// 卸载分区
const handleUmount = (path: string) => {
  window.$dialog.warning({
    title: $gettext('Confirm'),
    content: $gettext('Are you sure you want to unmount this partition?'),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.umount(path)).onSuccess(() => {
        window.$message.success($gettext('Unmounted successfully'))
        loadDiskList()
      })
    }
  })
}

// 格式化分区
const handleFormat = () => {
  if (!formatDevice.value) {
    window.$message.error($gettext('Please select a device'))
    return
  }

  window.$dialog.error({
    title: $gettext('Dangerous Operation'),
    content: $gettext(
      'Formatting will erase all data on the partition. This operation is irreversible. Are you sure?'
    ),
    positiveText: $gettext('Confirm Format'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.format(formatDevice.value, formatFsType.value)).onSuccess(() => {
        window.$message.success($gettext('Formatted successfully'))
        loadDiskList()
        formatDevice.value = ''
        formatFsType.value = 'ext4'
      })
    }
  })
}

// 初始化磁盘
const handleInit = () => {
  if (!initDevice.value) {
    window.$message.error($gettext('Please enter disk name'))
    return
  }

  window.$dialog.error({
    title: $gettext('Dangerous Operation'),
    content: $gettext(
      'This will delete all partitions on %{ device } and create a single partition. All data will be permanently lost. Are you absolutely sure?',
      { device: initDevice.value }
    ),
    positiveText: $gettext('Confirm Initialize'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.init(initDevice.value, initFsType.value)).onSuccess(() => {
        window.$message.success($gettext('Disk initialized successfully'))
        loadDiskList()
        initDevice.value = ''
        initFsType.value = 'ext4'
      })
    }
  })
}

// 创建物理卷
const handleCreatePV = () => {
  if (!pvDevice.value) {
    window.$message.error($gettext('Please select a device'))
    return
  }

  window.$dialog.warning({
    title: $gettext('Confirm'),
    content: $gettext('Are you sure you want to create a physical volume on %{ device }?', {
      device: pvDevice.value
    }),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.createPV(pvDevice.value)).onSuccess(() => {
        window.$message.success($gettext('Physical volume created successfully'))
        loadLVMInfo()
        pvDevice.value = ''
      })
    }
  })
}

// 删除物理卷
const handleRemovePV = (device: string) => {
  window.$dialog.error({
    title: $gettext('Dangerous Operation'),
    content: $gettext('Are you sure you want to remove the physical volume %{ device }?', {
      device
    }),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.removePV(device)).onSuccess(() => {
        window.$message.success($gettext('Physical volume removed successfully'))
        loadLVMInfo()
      })
    }
  })
}

// 创建卷组
const handleCreateVG = () => {
  if (!vgName.value || vgDevices.value.length === 0) {
    window.$message.error($gettext('Please fill in all fields'))
    return
  }

  window.$dialog.warning({
    title: $gettext('Confirm'),
    content: $gettext('Are you sure you want to create volume group %{ name }?', {
      name: vgName.value
    }),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.createVG(vgName.value, vgDevices.value)).onSuccess(() => {
        window.$message.success($gettext('Volume group created successfully'))
        loadLVMInfo()
        vgName.value = ''
        vgDevices.value = []
      })
    }
  })
}

// 删除卷组
const handleRemoveVG = (name: string) => {
  window.$dialog.error({
    title: $gettext('Dangerous Operation'),
    content: $gettext(
      'Are you sure you want to remove the volume group %{ name }? All logical volumes in this group will be deleted!',
      { name }
    ),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.removeVG(name)).onSuccess(() => {
        window.$message.success($gettext('Volume group removed successfully'))
        loadLVMInfo()
      })
    }
  })
}

// 创建逻辑卷
const handleCreateLV = () => {
  if (!lvName.value || !lvVgName.value || lvSize.value < 1) {
    window.$message.error($gettext('Please fill in all fields'))
    return
  }

  window.$dialog.warning({
    title: $gettext('Confirm'),
    content: $gettext(
      'Are you sure you want to create logical volume %{ name } with %{ size }GB?',
      {
        name: lvName.value,
        size: lvSize.value
      }
    ),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.createLV(lvName.value, lvVgName.value, lvSize.value)).onSuccess(() => {
        window.$message.success($gettext('Logical volume created successfully'))
        loadLVMInfo()
        lvName.value = ''
        lvVgName.value = ''
        lvSize.value = 1
      })
    }
  })
}

// 删除逻辑卷
const handleRemoveLV = (path: string) => {
  window.$dialog.error({
    title: $gettext('Dangerous Operation'),
    content: $gettext(
      'Are you sure you want to remove the logical volume %{ path }? All data on this volume will be lost!',
      { path }
    ),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.removeLV(path)).onSuccess(() => {
        window.$message.success($gettext('Logical volume removed successfully'))
        loadLVMInfo()
      })
    }
  })
}

// 扩容逻辑卷
const handleExtendLV = () => {
  if (!extendLvPath.value || extendSize.value < 1) {
    window.$message.error($gettext('Please fill in all fields'))
    return
  }

  window.$dialog.warning({
    title: $gettext('Confirm'),
    content: $gettext('Are you sure you want to extend %{ path } by %{ size }GB?', {
      path: extendLvPath.value,
      size: extendSize.value
    }),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.extendLV(extendLvPath.value, extendSize.value, extendResize.value)).onSuccess(
        () => {
          window.$message.success($gettext('Logical volume extended successfully'))
          loadLVMInfo()
          extendLvPath.value = ''
          extendSize.value = 1
        }
      )
    }
  })
}

// 删除 fstab 条目
const handleDeleteFstab = (mountPoint: string) => {
  window.$dialog.error({
    title: $gettext('Dangerous Operation'),
    content: $gettext(
      'Are you sure you want to remove the fstab entry for %{ mountPoint }? This will prevent auto-mount on boot.',
      { mountPoint }
    ),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.fstabDelete(mountPoint)).onSuccess(() => {
        window.$message.success($gettext('Fstab entry removed successfully'))
        loadFstabList()
      })
    }
  })
}
</script>

<template>
  <n-tabs v-model:value="currentTab" type="line" placement="left" animated>
    <!-- 磁盘管理标签页 -->
    <n-tab-pane name="disk" :tab="$gettext('Disk Management')">
      <n-flex vertical :size="16">
        <!-- 磁盘卡片列表 -->
        <n-card v-for="diskItem in diskList" :key="diskItem.name">
          <template #header>
            <n-flex align="center" :size="12">
              <span style="font-weight: 600">{{ $gettext('Disk Name') }}: {{ diskItem.name }}</span>
              <n-tag v-if="diskItem.isSystemDisk" type="error" size="small">
                {{ $gettext('System Disk') }}
              </n-tag>
            </n-flex>
          </template>
          <template #header-extra>
            <n-flex align="center" :size="16">
              <span>{{ $gettext('Size') }}: {{ formatBytes(diskItem.size) }}</span>
              <span>{{ $gettext('Partitions') }}: {{ diskItem.partitions.length }}</span>
              <span>{{ $gettext('Disk Type') }}:</span>
              <n-tag size="small">{{ getDiskTypeLabel(diskItem.model) }}</n-tag>
            </n-flex>
          </template>

          <n-data-table
            :columns="partitionColumns"
            :data="diskItem.partitions"
            :bordered="false"
            :single-line="false"
            size="small"
            :row-key="(row: PartitionData) => row.name"
          />

          <n-alert
            v-if="diskItem.isSystemDisk"
            type="warning"
            :show-icon="false"
            style="margin-top: 12px"
          >
            {{ $gettext('Note: This is the system disk and cannot be operated on.') }}
          </n-alert>
        </n-card>

        <!-- 无磁盘时显示 -->
        <n-empty v-if="diskList.length === 0" :description="$gettext('No disks found')" />

        <!-- 挂载分区 -->
        <n-card :title="$gettext('Mount Partition')">
          <n-form>
            <n-flex :size="16" :wrap="true">
              <n-form-item :label="$gettext('Partition')">
                <n-select
                  v-model:value="selectedDevice"
                  :options="unmountedPartitionOptions"
                  :placeholder="$gettext('Select partition')"
                  style="width: 200px"
                  filterable
                />
              </n-form-item>
              <n-form-item :label="$gettext('Mount Path')">
                <n-input
                  v-model:value="mountPath"
                  :placeholder="$gettext('e.g., /mnt/data')"
                  style="width: 200px"
                />
              </n-form-item>
              <n-form-item :label="$gettext('Mount Options')">
                <n-input
                  v-model:value="mountOption"
                  :placeholder="$gettext('e.g., defaults,noatime')"
                  style="width: 200px"
                />
              </n-form-item>
              <n-form-item :label="$gettext('Auto-mount on boot')">
                <n-switch v-model:value="mountWriteFstab" />
              </n-form-item>
              <n-form-item>
                <n-button type="primary" @click="handleMount">{{ $gettext('Mount') }}</n-button>
              </n-form-item>
            </n-flex>
          </n-form>
          <n-alert v-if="mountWriteFstab" type="info" style="margin-top: 12px">
            {{
              $gettext(
                'When enabled, the partition UUID will be written to /etc/fstab for automatic mounting on system boot.'
              )
            }}
          </n-alert>
        </n-card>
        <!-- 格式化分区 -->
        <n-card :title="$gettext('Format Partition')">
          <n-alert type="error" style="margin-bottom: 16px">
            {{ $gettext('Warning: Formatting will erase all data!') }}
          </n-alert>
          <n-form inline>
            <n-form-item :label="$gettext('Partition')">
              <n-select
                v-model:value="formatDevice"
                :options="unmountedPartitionOptions"
                :placeholder="$gettext('Select partition')"
                style="width: 200px"
                filterable
              />
            </n-form-item>
            <n-form-item :label="$gettext('Filesystem Type')">
              <n-select
                v-model:value="formatFsType"
                :options="fsTypeOptions"
                style="width: 150px"
              />
            </n-form-item>
            <n-form-item>
              <n-button type="error" @click="handleFormat">
                {{ $pgettext('disk action', 'Format') }}
              </n-button>
            </n-form-item>
          </n-form>
        </n-card>
        <!-- 初始化磁盘 -->
        <n-card :title="$gettext('Initialize Disk')">
          <n-alert type="error" style="margin-bottom: 16px">
            {{
              $gettext(
                'Warning: This will delete all partitions and create a single partition. All data will be lost!'
              )
            }}
          </n-alert>
          <n-form inline>
            <n-form-item :label="$gettext('Disk')">
              <n-select
                v-model:value="initDevice"
                :options="nonSystemDiskOptions"
                :placeholder="$gettext('Select disk')"
                style="width: 200px"
                filterable
              />
            </n-form-item>
            <n-form-item :label="$gettext('Filesystem Type')">
              <n-select v-model:value="initFsType" :options="fsTypeOptions" style="width: 150px" />
            </n-form-item>
            <n-form-item>
              <n-button type="error" @click="handleInit">{{ $gettext('Initialize') }}</n-button>
            </n-form-item>
          </n-form>
        </n-card>
        <!-- 开机自动挂载 (fstab) -->
        <n-card :title="$gettext('Auto-mount Configuration (fstab)')">
          <n-space vertical>
            <n-data-table
              v-if="fstabList.length > 0"
              :columns="[
                { title: $gettext('Device'), key: 'device', ellipsis: { tooltip: true } },
                { title: $gettext('Mount Point'), key: 'mount_point' },
                { title: $gettext('Filesystem'), key: 'fs_type', width: 100 },
                { title: $gettext('Options'), key: 'options', ellipsis: { tooltip: true } },
                {
                  title: $gettext('Actions'),
                  key: 'actions',
                  width: 100,
                  render(row: FstabEntry) {
                    // 不允许删除根目录挂载
                    if (row.mount_point === '/') return null
                    return h(
                      NButton,
                      {
                        size: 'small',
                        type: 'error',
                        onClick: () => handleDeleteFstab(row.mount_point)
                      },
                      { default: () => $gettext('Remove') }
                    )
                  }
                }
              ]"
              :data="fstabList"
              :bordered="false"
              size="small"
              :row-key="(row: FstabEntry) => row.mount_point"
            />
            <n-empty v-else :description="$gettext('No fstab entries')" />
          </n-space>
        </n-card>
      </n-flex>
    </n-tab-pane>

    <!-- LVM管理标签页 -->
    <n-tab-pane name="lvm" :tab="$gettext('LVM Management')">
      <n-flex vertical>
        <n-card :title="$gettext('Physical Volumes')">
          <n-space vertical>
            <n-list v-if="lvmInfo.pvs && lvmInfo.pvs.length > 0" bordered>
              <n-list-item v-for="(pv, index) in lvmInfo.pvs" :key="index">
                <n-thing>
                  <template #header>{{ pv.field_0 }}</template>
                  <template #description>
                    VG: {{ pv.field_1 }} | Size: {{ pv.field_2 }} | Free: {{ pv.field_3 }}
                  </template>
                  <template #action>
                    <n-button size="small" type="error" @click="handleRemovePV(pv.field_0)">
                      {{ $gettext('Remove') }}
                    </n-button>
                  </template>
                </n-thing>
              </n-list-item>
            </n-list>
            <n-empty v-else :description="$gettext('No physical volumes')" />

            <n-divider />
            <n-form>
              <n-form-item :label="$gettext('Device')">
                <n-select
                  v-model:value="pvDevice"
                  :options="availablePVDeviceOptions"
                  :placeholder="$gettext('Select device')"
                  filterable
                  style="width: 300px"
                />
              </n-form-item>
              <n-button type="primary" @click="handleCreatePV">
                {{ $gettext('Create PV') }}
              </n-button>
            </n-form>
          </n-space>
        </n-card>

        <n-card :title="$gettext('Volume Groups')">
          <n-space vertical>
            <n-list v-if="lvmInfo.vgs && lvmInfo.vgs.length > 0" bordered>
              <n-list-item v-for="(vg, index) in lvmInfo.vgs" :key="index">
                <n-thing>
                  <template #header>{{ vg.field_0 }}</template>
                  <template #description>
                    PV: {{ vg.field_1 }} | LV: {{ vg.field_2 }} | Size: {{ vg.field_3 }} | Free:
                    {{ vg.field_4 }}
                  </template>
                  <template #action>
                    <n-button size="small" type="error" @click="handleRemoveVG(vg.field_0)">
                      {{ $gettext('Remove') }}
                    </n-button>
                  </template>
                </n-thing>
              </n-list-item>
            </n-list>
            <n-empty v-else :description="$gettext('No volume groups')" />

            <n-divider />
            <n-form>
              <n-form-item :label="$gettext('VG Name')">
                <n-input
                  v-model:value="vgName"
                  :placeholder="$gettext('Enter VG name')"
                  style="width: 300px"
                />
              </n-form-item>
              <n-form-item :label="$gettext('Physical Volumes')">
                <n-select
                  v-model:value="vgDevices"
                  :options="availablePVOptions"
                  :placeholder="$gettext('Select PVs')"
                  multiple
                  filterable
                  style="width: 400px"
                />
              </n-form-item>
              <n-button type="primary" @click="handleCreateVG">
                {{ $gettext('Create VG') }}
              </n-button>
            </n-form>
          </n-space>
        </n-card>

        <n-card :title="$gettext('Logical Volumes')">
          <n-space vertical>
            <n-list v-if="lvmInfo.lvs && lvmInfo.lvs.length > 0" bordered>
              <n-list-item v-for="(lv, index) in lvmInfo.lvs" :key="index">
                <n-thing>
                  <template #header>{{ lv.field_0 }}</template>
                  <template #description>
                    VG: {{ lv.field_1 }} | Size: {{ lv.field_2 }} | Path: {{ lv.field_3 }}
                  </template>
                  <template #action>
                    <n-button size="small" type="error" @click="handleRemoveLV(lv.field_3)">
                      {{ $gettext('Remove') }}
                    </n-button>
                  </template>
                </n-thing>
              </n-list-item>
            </n-list>
            <n-empty v-else :description="$gettext('No logical volumes')" />

            <n-divider />
            <n-form>
              <n-form-item :label="$gettext('LV Name')">
                <n-input
                  v-model:value="lvName"
                  :placeholder="$gettext('Enter LV name')"
                  style="width: 300px"
                />
              </n-form-item>
              <n-form-item :label="$gettext('Volume Group')">
                <n-select
                  v-model:value="lvVgName"
                  :options="vgOptions"
                  :placeholder="$gettext('Select VG')"
                  filterable
                  style="width: 300px"
                />
              </n-form-item>
              <n-form-item :label="$gettext('Size (GB)')">
                <n-input-number v-model:value="lvSize" :min="1" />
              </n-form-item>
              <n-button type="primary" @click="handleCreateLV">
                {{ $gettext('Create LV') }}
              </n-button>
            </n-form>
          </n-space>
        </n-card>

        <n-card :title="$gettext('Extend Logical Volume')">
          <n-form>
            <n-form-item :label="$gettext('Logical Volume')">
              <n-select
                v-model:value="extendLvPath"
                :options="lvOptions"
                :placeholder="$gettext('Select LV')"
                filterable
                style="width: 400px"
              />
            </n-form-item>
            <n-form-item :label="$gettext('Extend Size (GB)')">
              <n-input-number v-model:value="extendSize" :min="1" />
            </n-form-item>
            <n-form-item :label="$gettext('Auto Resize Filesystem')">
              <n-switch v-model:value="extendResize" />
            </n-form-item>
            <n-button type="primary" @click="handleExtendLV">
              {{ $gettext('Extend LV') }}
            </n-button>
          </n-form>
        </n-card>
      </n-flex>
    </n-tab-pane>
  </n-tabs>
</template>

<style scoped lang="scss"></style>

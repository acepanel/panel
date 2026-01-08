<script setup lang="ts">
defineOptions({
  name: 'toolbox-disk'
})

import { useRequest } from 'alova/client'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NProgress, NTag } from 'naive-ui'
import { computed, onMounted, ref } from 'vue'
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

const { $gettext } = useGettext()
const currentTab = ref('disk')
const diskList = ref<DiskData[]>([])
const lvmInfo = ref<any>({ pvs: [], vgs: [], lvs: [] })

// 磁盘管理
const selectedDevice = ref('')
const mountPath = ref('')
const formatDevice = ref('')
const formatFsType = ref('ext4')
const fsTypeOptions = [
  { label: 'ext4', value: 'ext4' },
  { label: 'ext3', value: 'ext3' },
  { label: 'xfs', value: 'xfs' },
  { label: 'btrfs', value: 'btrfs' }
]

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
  if (!model) return 'HDD'
  const modelLower = model.toLowerCase()
  if (modelLower.includes('ssd') || modelLower.includes('nvme')) {
    return 'SSD'
  }
  return 'HDD'
}

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

onMounted(() => {
  loadDiskList()
  loadLVMInfo()
})

// 挂载分区
const handleMount = () => {
  if (!selectedDevice.value || !mountPath.value) {
    window.$message.error($gettext('Please fill in all fields'))
    return
  }

  useRequest(disk.mount(selectedDevice.value, mountPath.value)).onSuccess(() => {
    window.$message.success($gettext('Mounted successfully'))
    loadDiskList()
    selectedDevice.value = ''
    mountPath.value = ''
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

// 创建物理卷
const handleCreatePV = () => {
  if (!pvDevice.value) {
    window.$message.error($gettext('Please enter device name'))
    return
  }

  useRequest(disk.createPV(pvDevice.value)).onSuccess(() => {
    window.$message.success($gettext('Physical volume created successfully'))
    loadLVMInfo()
    pvDevice.value = ''
  })
}

// 删除物理卷
const handleRemovePV = (device: string) => {
  window.$dialog.warning({
    title: $gettext('Confirm'),
    content: $gettext('Are you sure you want to remove this physical volume?'),
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

  useRequest(disk.createVG(vgName.value, vgDevices.value)).onSuccess(() => {
    window.$message.success($gettext('Volume group created successfully'))
    loadLVMInfo()
    vgName.value = ''
    vgDevices.value = []
  })
}

// 删除卷组
const handleRemoveVG = (name: string) => {
  window.$dialog.warning({
    title: $gettext('Confirm'),
    content: $gettext('Are you sure you want to remove this volume group?'),
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

  useRequest(disk.createLV(lvName.value, lvVgName.value, lvSize.value)).onSuccess(() => {
    window.$message.success($gettext('Logical volume created successfully'))
    loadLVMInfo()
    lvName.value = ''
    lvVgName.value = ''
    lvSize.value = 1
  })
}

// 删除逻辑卷
const handleRemoveLV = (path: string) => {
  window.$dialog.warning({
    title: $gettext('Confirm'),
    content: $gettext('Are you sure you want to remove this logical volume?'),
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

  useRequest(disk.extendLV(extendLvPath.value, extendSize.value, extendResize.value)).onSuccess(
    () => {
      window.$message.success($gettext('Logical volume extended successfully'))
      loadLVMInfo()
      extendLvPath.value = ''
      extendSize.value = 1
    }
  )
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
          <n-form inline>
            <n-form-item :label="$gettext('Device')">
              <n-input
                v-model:value="selectedDevice"
                :placeholder="$gettext('e.g., sdb1')"
                style="width: 200px"
              />
            </n-form-item>
            <n-form-item :label="$gettext('Mount Path')">
              <n-input
                v-model:value="mountPath"
                :placeholder="$gettext('e.g., /mnt/data')"
                style="width: 200px"
              />
            </n-form-item>
            <n-form-item>
              <n-button type="primary" @click="handleMount">{{ $gettext('Mount') }}</n-button>
            </n-form-item>
          </n-form>
        </n-card>

        <!-- 格式化分区 -->
        <n-card :title="$gettext('Format Partition')">
          <n-alert type="error" style="margin-bottom: 16px">
            {{ $gettext('Warning: Formatting will erase all data!') }}
          </n-alert>
          <n-form inline>
            <n-form-item :label="$gettext('Device')">
              <n-input
                v-model:value="formatDevice"
                :placeholder="$gettext('e.g., sdb1')"
                style="width: 200px"
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
              <n-button type="error" @click="handleFormat">{{ $gettext('Format') }}</n-button>
            </n-form-item>
          </n-form>
        </n-card>
      </n-flex>
    </n-tab-pane>

    <!-- LVM管理标签页 -->
    <n-tab-pane name="lvm" :tab="$gettext('LVM Management')">
      <n-flex vertical>
        <n-card :title="$gettext('Physical Volumes')">
          <n-space vertical>
            <n-button @click="loadLVMInfo">{{ $gettext('Refresh') }}</n-button>
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
                <n-input v-model:value="pvDevice" :placeholder="$gettext('e.g., sdb')" />
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
                <n-input v-model:value="vgName" :placeholder="$gettext('Enter VG name')" />
              </n-form-item>
              <n-form-item :label="$gettext('Devices')">
                <n-dynamic-tags v-model:value="vgDevices" />
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
                <n-input v-model:value="lvName" :placeholder="$gettext('Enter LV name')" />
              </n-form-item>
              <n-form-item :label="$gettext('VG Name')">
                <n-input v-model:value="lvVgName" :placeholder="$gettext('Enter VG name')" />
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
            <n-form-item :label="$gettext('LV Path')">
              <n-input v-model:value="extendLvPath" :placeholder="$gettext('e.g., /dev/vg0/lv0')" />
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

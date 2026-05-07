<script setup lang="ts">
defineOptions({
  name: 'toolbox-disk',
})

import { useRequest } from 'alova/client'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NProgress, NTag } from 'naive-ui'
import { h } from 'vue'
import { useGettext } from 'vue3-gettext'

import disk from '@/api/panel/toolbox-disk'
import { formatBytes } from '@/utils'

import RaidView from './disk/RaidView.vue'
import SmartView from './disk/SmartView.vue'

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
const diskOpTab = ref('mount')
const lvmOpTab = ref('pv')
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
  { label: 'btrfs', value: 'btrfs' },
]

interface FstabEntry {
  device: string
  mount_point: string
  fs_type: string
  options: string
  dump: string
  pass: string
}
const fstabList = ref<FstabEntry[]>([])

// LVM 管理
const pvDevice = ref('')
const vgName = ref('')
const vgDevices = ref<string[]>([])
const lvName = ref('')
const lvVgName = ref('')
const lvSize = ref(1)
const extendLvPath = ref('')
const extendSize = ref(1)
const extendResize = ref(true)

interface DfInfo {
  size: string
  used: string
  avail: string
  percent: string
}

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

const parseDiskData = (devices: BlockDevice[], dfData: Record<string, DfInfo>): DiskData[] => {
  const disks: DiskData[] = []
  for (const device of devices) {
    if (device.type !== 'disk') continue
    const partitions: PartitionData[] = []
    let isSystemDisk = false
    if (device.children) {
      for (const child of device.children) {
        if (child.type === 'part' && child.mountpoint === '/') {
          isSystemDisk = true
          break
        }
      }
    }
    if (device.children) {
      for (const child of device.children) {
        if (child.type === 'part') {
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
            isSystemDisk,
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
      partitions,
    })
  }
  return disks
}

const getDiskTypeLabel = (model: string | null): string => {
  if (!model) return $gettext('Unknown')
  const modelLower = model.toLowerCase()
  if (modelLower.includes('ssd') || modelLower.includes('nvme')) return 'SSD'
  return model.toUpperCase()
}

const unmountedPartitionOptions = computed(() => {
  const options: { label: string; value: string }[] = []
  for (const d of diskList.value) {
    for (const part of d.partitions) {
      if (!part.mountpoint) {
        options.push({
          label: `${part.name} (${formatBytes(part.size)})`,
          value: part.name,
        })
      }
    }
  }
  return options
})

const nonSystemDiskOptions = computed(() => {
  return diskList.value
    .filter((d) => !d.isSystemDisk)
    .map((d) => ({
      label: `${d.name} (${formatBytes(d.size)})`,
      value: d.name,
    }))
})

const availablePVDeviceOptions = computed(() => {
  const options: { label: string; value: string }[] = []
  const existingPVs = new Set(lvmInfo.value.pvs?.map((pv: any) => pv.field_0) || [])
  for (const d of diskList.value) {
    if (d.isSystemDisk) continue
    if (d.partitions.length === 0) {
      const devPath = `/dev/${d.name}`
      if (!existingPVs.has(devPath)) {
        options.push({ label: `${d.name} (${formatBytes(d.size)})`, value: d.name })
      }
    }
    for (const part of d.partitions) {
      const devPath = `/dev/${part.name}`
      if (!part.mountpoint && !existingPVs.has(devPath)) {
        options.push({ label: `${part.name} (${formatBytes(part.size)})`, value: part.name })
      }
    }
  }
  return options
})

const availablePVOptions = computed(() => {
  return (lvmInfo.value.pvs || [])
    .filter((pv: any) => !pv.field_1)
    .map((pv: any) => ({ label: `${pv.field_0} (${pv.field_2})`, value: pv.field_0 }))
})

const vgOptions = computed(() => {
  return (lvmInfo.value.vgs || []).map((vg: any) => ({
    label: `${vg.field_0} (${$gettext('Free')}: ${vg.field_4})`,
    value: vg.field_0,
  }))
})

const lvOptions = computed(() => {
  return (lvmInfo.value.lvs || []).map((lv: any) => ({
    label: `${lv.field_0} (${lv.field_2}) - ${lv.field_3}`,
    value: lv.field_3,
  }))
})

const partitionColumns = computed<DataTableColumns<PartitionData>>(() => [
  { title: $gettext('Partition'), key: 'name', width: 180 },
  {
    title: $gettext('Size'),
    key: 'size',
    width: 110,
    render(row) {
      return formatBytes(row.size)
    },
  },
  {
    title: $gettext('Used'),
    key: 'used',
    width: 110,
    render(row) {
      if (!row.mountpoint) return '-'
      return formatBytes(row.used)
    },
  },
  {
    title: $gettext('Available'),
    key: 'available',
    width: 110,
    render(row) {
      if (!row.mountpoint) return '-'
      return formatBytes(row.available)
    },
  },
  {
    title: $gettext('Usage'),
    key: 'usagePercent',
    width: 160,
    render(row) {
      if (!row.mountpoint) {
        return h(
          NTag,
          { type: 'warning', size: 'small', bordered: false },
          { default: () => $gettext('Not Mounted') },
        )
      }
      const percent = row.usagePercent
      const status = percent > 90 ? 'error' : percent > 70 ? 'warning' : 'success'
      return h(NProgress, {
        type: 'line',
        percentage: percent,
        status,
        indicatorPlacement: 'inside',
        style: { width: '120px' },
      })
    },
  },
  {
    title: $gettext('Mount Point'),
    key: 'mountpoint',
    minWidth: 180,
    render(row) {
      return row.mountpoint || '-'
    },
  },
  {
    title: $gettext('FS'),
    key: 'fstype',
    width: 90,
    render(row) {
      return row.fstype || '-'
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 110,
    render(row) {
      if (row.mountpoint) {
        return h(
          NButton,
          {
            size: 'small',
            type: 'warning',
            secondary: true,
            disabled: row.isSystemDisk,
            onClick: () => handleUmount(row.mountpoint!),
          },
          { default: () => $gettext('Unmount') },
        )
      }
      return null
    },
  },
])

const loadLVMInfo = () => {
  useRequest(disk.lvmInfo()).onSuccess(({ data }) => {
    lvmInfo.value = data
  })
}

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

const handleMount = () => {
  if (!selectedDevice.value || !mountPath.value) {
    window.$message.error($gettext('Please fill in all fields'))
    return
  }
  const confirmContent = mountWriteFstab.value
    ? $gettext(
        'Are you sure you want to mount %{ device } to %{ path } and write to fstab for auto-mount on boot?',
        { device: selectedDevice.value, path: mountPath.value },
      )
    : $gettext('Are you sure you want to mount %{ device } to %{ path }?', {
        device: selectedDevice.value,
        path: mountPath.value,
      })
  window.$dialog.warning({
    title: $gettext('Confirm'),
    content: confirmContent,
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(
        disk.mount(selectedDevice.value, mountPath.value, mountWriteFstab.value, mountOption.value),
      ).onSuccess(() => {
        window.$message.success($gettext('Mounted successfully'))
        loadDiskList()
        if (mountWriteFstab.value) loadFstabList()
        selectedDevice.value = ''
        mountPath.value = ''
        mountWriteFstab.value = false
        mountOption.value = ''
      })
    },
  })
}

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
    },
  })
}

const handleFormat = () => {
  if (!formatDevice.value) {
    window.$message.error($gettext('Please select a device'))
    return
  }
  window.$dialog.error({
    title: $gettext('Dangerous Operation'),
    content: $gettext(
      'Formatting will erase all data on the partition. This operation is irreversible. Are you sure?',
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
    },
  })
}

const handleInit = () => {
  if (!initDevice.value) {
    window.$message.error($gettext('Please enter disk name'))
    return
  }
  window.$dialog.error({
    title: $gettext('Dangerous Operation'),
    content: $gettext(
      'This will delete all partitions on %{ device } and create a single partition. All data will be permanently lost. Are you absolutely sure?',
      { device: initDevice.value },
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
    },
  })
}

const handleCreatePV = () => {
  if (!pvDevice.value) {
    window.$message.error($gettext('Please select a device'))
    return
  }
  window.$dialog.warning({
    title: $gettext('Confirm'),
    content: $gettext('Are you sure you want to create a physical volume on %{ device }?', {
      device: pvDevice.value,
    }),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.createPV(pvDevice.value)).onSuccess(() => {
        window.$message.success($gettext('Physical volume created successfully'))
        loadLVMInfo()
        pvDevice.value = ''
      })
    },
  })
}

const handleRemovePV = (device: string) => {
  window.$dialog.error({
    title: $gettext('Dangerous Operation'),
    content: $gettext('Are you sure you want to remove the physical volume %{ device }?', {
      device,
    }),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.removePV(device)).onSuccess(() => {
        window.$message.success($gettext('Physical volume removed successfully'))
        loadLVMInfo()
      })
    },
  })
}

const handleCreateVG = () => {
  if (!vgName.value || vgDevices.value.length === 0) {
    window.$message.error($gettext('Please fill in all fields'))
    return
  }
  window.$dialog.warning({
    title: $gettext('Confirm'),
    content: $gettext('Are you sure you want to create volume group %{ name }?', {
      name: vgName.value,
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
    },
  })
}

const handleRemoveVG = (name: string) => {
  window.$dialog.error({
    title: $gettext('Dangerous Operation'),
    content: $gettext(
      'Are you sure you want to remove the volume group %{ name }? All logical volumes in this group will be deleted!',
      { name },
    ),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.removeVG(name)).onSuccess(() => {
        window.$message.success($gettext('Volume group removed successfully'))
        loadLVMInfo()
      })
    },
  })
}

const handleCreateLV = () => {
  if (!lvName.value || !lvVgName.value || lvSize.value < 1) {
    window.$message.error($gettext('Please fill in all fields'))
    return
  }
  window.$dialog.warning({
    title: $gettext('Confirm'),
    content: $gettext(
      'Are you sure you want to create logical volume %{ name } with %{ size }GB?',
      { name: lvName.value, size: lvSize.value },
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
    },
  })
}

const handleRemoveLV = (path: string) => {
  window.$dialog.error({
    title: $gettext('Dangerous Operation'),
    content: $gettext(
      'Are you sure you want to remove the logical volume %{ path }? All data on this volume will be lost!',
      { path },
    ),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.removeLV(path)).onSuccess(() => {
        window.$message.success($gettext('Logical volume removed successfully'))
        loadLVMInfo()
      })
    },
  })
}

const handleExtendLV = () => {
  if (!extendLvPath.value || extendSize.value < 1) {
    window.$message.error($gettext('Please fill in all fields'))
    return
  }
  window.$dialog.warning({
    title: $gettext('Confirm'),
    content: $gettext('Are you sure you want to extend %{ path } by %{ size }GB?', {
      path: extendLvPath.value,
      size: extendSize.value,
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
        },
      )
    },
  })
}

const handleDeleteFstab = (mountPoint: string) => {
  window.$dialog.error({
    title: $gettext('Dangerous Operation'),
    content: $gettext(
      'Are you sure you want to remove the fstab entry for %{ mountPoint }? This will prevent auto-mount on boot.',
      { mountPoint },
    ),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(disk.fstabDelete(mountPoint)).onSuccess(() => {
        window.$message.success($gettext('Fstab entry removed successfully'))
        loadFstabList()
      })
    },
  })
}

const fstabColumns = computed<DataTableColumns<FstabEntry>>(() => [
  { title: $gettext('Device'), key: 'device', ellipsis: { tooltip: true } },
  { title: $gettext('Mount Point'), key: 'mount_point', minWidth: 160 },
  { title: $gettext('FS'), key: 'fs_type', width: 100 },
  { title: $gettext('Options'), key: 'options', ellipsis: { tooltip: true } },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 100,
    render(row: FstabEntry) {
      if (row.mount_point === '/') return null
      return h(
        NButton,
        {
          size: 'small',
          type: 'error',
          secondary: true,
          onClick: () => handleDeleteFstab(row.mount_point),
        },
        { default: () => $gettext('Remove') },
      )
    },
  },
])
</script>

<template>
  <n-tabs v-model:value="currentTab" type="line" placement="left" animated>
    <!-- 磁盘管理 -->
    <n-tab-pane name="disk" :tab="$gettext('Disk Management')">
      <n-flex vertical :size="16">
        <!-- 磁盘卡片列表 -->
        <div v-if="diskList.length > 0" class="disk-list">
          <div v-for="diskItem in diskList" :key="diskItem.name" class="disk-card">
            <div class="disk-card__head">
              <div class="disk-card__title">
                <i-mdi-harddisk class="text-lg text-brand" />
                <span class="disk-card__name">{{ diskItem.name }}</span>
                <n-tag v-if="diskItem.isSystemDisk" type="error" size="small" :bordered="false">
                  {{ $gettext('System Disk') }}
                </n-tag>
                <n-tag size="small" :bordered="false">
                  {{ getDiskTypeLabel(diskItem.model) }}
                </n-tag>
              </div>
              <div class="disk-card__meta">
                <span>{{ formatBytes(diskItem.size) }}</span>
                <span class="disk-card__sep">·</span>
                <span>
                  {{
                    $gettext('%{ count } partitions', {
                      count: diskItem.partitions.length.toString(),
                    })
                  }}
                </span>
              </div>
            </div>
            <n-data-table
              :columns="partitionColumns"
              :data="diskItem.partitions"
              :bordered="false"
              :single-line="false"
              size="small"
              :row-key="(row: PartitionData) => row.name"
            />
          </div>
        </div>
        <n-empty v-else :description="$gettext('No disks found')" />

        <!-- 操作面板 -->
        <n-tabs
          v-model:value="diskOpTab"
          type="line"
          animated
          size="small"
          :theme-overrides="{
            tabGapSmallLine: '20px',
            tabPaddingSmallLine: '8px 0',
          }"
          pane-style="padding-top: 16px;"
        >
          <n-tab-pane name="mount" :tab="$gettext('Mount')">
            <n-flex vertical :size="12">
              <n-grid :x-gap="12" :y-gap="12" cols="1 m:2" responsive="screen">
                <n-gi>
                  <div class="form-label">{{ $gettext('Partition') }}</div>
                  <n-select
                    v-model:value="selectedDevice"
                    :options="unmountedPartitionOptions"
                    :placeholder="$gettext('Select partition')"
                    filterable
                  />
                </n-gi>
                <n-gi>
                  <div class="form-label">{{ $gettext('Mount Path') }}</div>
                  <n-input v-model:value="mountPath" :placeholder="$gettext('e.g., /mnt/data')" />
                </n-gi>
                <n-gi>
                  <div class="form-label">{{ $gettext('Mount Options') }}</div>
                  <n-input
                    v-model:value="mountOption"
                    :placeholder="$gettext('e.g., defaults,noatime')"
                  />
                </n-gi>
                <n-gi>
                  <div class="form-label">{{ $gettext('Auto-mount on boot') }}</div>
                  <n-switch v-model:value="mountWriteFstab" />
                </n-gi>
              </n-grid>
              <n-flex justify="end">
                <n-button type="primary" @click="handleMount">
                  {{ $gettext('Mount') }}
                </n-button>
              </n-flex>
            </n-flex>
          </n-tab-pane>

          <n-tab-pane name="format" :tab="$pgettext('disk action', 'Format')">
            <n-flex vertical :size="12">
              <n-alert type="error">
                {{ $gettext('Warning: Formatting will erase all data!') }}
              </n-alert>
              <n-grid :x-gap="12" :y-gap="12" cols="1 m:2" responsive="screen">
                <n-gi>
                  <div class="form-label">{{ $gettext('Partition') }}</div>
                  <n-select
                    v-model:value="formatDevice"
                    :options="unmountedPartitionOptions"
                    :placeholder="$gettext('Select partition')"
                    filterable
                  />
                </n-gi>
                <n-gi>
                  <div class="form-label">{{ $gettext('Filesystem Type') }}</div>
                  <n-select v-model:value="formatFsType" :options="fsTypeOptions" />
                </n-gi>
              </n-grid>
              <n-flex justify="end">
                <n-button type="error" @click="handleFormat">
                  {{ $pgettext('disk action', 'Format') }}
                </n-button>
              </n-flex>
            </n-flex>
          </n-tab-pane>

          <n-tab-pane name="init" :tab="$gettext('Initialize')">
            <n-flex vertical :size="12">
              <n-alert type="error">
                {{
                  $gettext(
                    'Warning: This will delete all partitions and create a single partition. All data will be lost!',
                  )
                }}
              </n-alert>
              <n-grid :x-gap="12" :y-gap="12" cols="1 m:2" responsive="screen">
                <n-gi>
                  <div class="form-label">{{ $gettext('Disk') }}</div>
                  <n-select
                    v-model:value="initDevice"
                    :options="nonSystemDiskOptions"
                    :placeholder="$gettext('Select disk')"
                    filterable
                  />
                </n-gi>
                <n-gi>
                  <div class="form-label">{{ $gettext('Filesystem Type') }}</div>
                  <n-select v-model:value="initFsType" :options="fsTypeOptions" />
                </n-gi>
              </n-grid>
              <n-flex justify="end">
                <n-button type="error" @click="handleInit">
                  {{ $gettext('Initialize') }}
                </n-button>
              </n-flex>
            </n-flex>
          </n-tab-pane>

          <n-tab-pane name="fstab" :tab="$gettext('Auto-mount (fstab)')">
            <n-data-table
              v-if="fstabList.length > 0"
              :columns="fstabColumns"
              :data="fstabList"
              :bordered="false"
              size="small"
              :row-key="(row: FstabEntry) => row.mount_point"
            />
            <n-empty v-else :description="$gettext('No fstab entries')" />
          </n-tab-pane>
        </n-tabs>
      </n-flex>
    </n-tab-pane>

    <!-- LVM 管理 -->
    <n-tab-pane name="lvm" :tab="$gettext('LVM Management')">
      <n-flex vertical :size="16">
        <!-- PV / VG / LV 概览 -->
        <n-grid :x-gap="16" :y-gap="16" cols="1 l:3" responsive="screen">
          <!-- PVs -->
          <n-gi>
            <div class="lvm-card">
              <div class="lvm-card__head">
                <span class="lvm-card__title">{{ $gettext('Physical Volumes') }}</span>
                <n-tag size="small" :bordered="false">
                  {{ (lvmInfo.pvs || []).length }}
                </n-tag>
              </div>
              <div class="lvm-card__body">
                <div v-for="(pv, index) in lvmInfo.pvs || []" :key="index" class="lvm-item">
                  <div class="lvm-item__main">
                    <div class="lvm-item__name">{{ pv.field_0 }}</div>
                    <div class="lvm-item__meta">
                      VG: {{ pv.field_1 || '-' }} · {{ pv.field_2 }}
                      <template v-if="pv.field_3">· Free: {{ pv.field_3 }}</template>
                    </div>
                  </div>
                  <n-button size="tiny" quaternary type="error" @click="handleRemovePV(pv.field_0)">
                    <template #icon>
                      <i-mdi-close />
                    </template>
                  </n-button>
                </div>
                <n-empty
                  v-if="!(lvmInfo.pvs || []).length"
                  size="small"
                  :description="$gettext('No physical volumes')"
                />
              </div>
            </div>
          </n-gi>

          <!-- VGs -->
          <n-gi>
            <div class="lvm-card">
              <div class="lvm-card__head">
                <span class="lvm-card__title">{{ $gettext('Volume Groups') }}</span>
                <n-tag size="small" :bordered="false">
                  {{ (lvmInfo.vgs || []).length }}
                </n-tag>
              </div>
              <div class="lvm-card__body">
                <div v-for="(vg, index) in lvmInfo.vgs || []" :key="index" class="lvm-item">
                  <div class="lvm-item__main">
                    <div class="lvm-item__name">{{ vg.field_0 }}</div>
                    <div class="lvm-item__meta">
                      PV: {{ vg.field_1 }} · LV: {{ vg.field_2 }} · {{ vg.field_3 }} · Free:
                      {{ vg.field_4 }}
                    </div>
                  </div>
                  <n-button size="tiny" quaternary type="error" @click="handleRemoveVG(vg.field_0)">
                    <template #icon>
                      <i-mdi-close />
                    </template>
                  </n-button>
                </div>
                <n-empty
                  v-if="!(lvmInfo.vgs || []).length"
                  size="small"
                  :description="$gettext('No volume groups')"
                />
              </div>
            </div>
          </n-gi>

          <!-- LVs -->
          <n-gi>
            <div class="lvm-card">
              <div class="lvm-card__head">
                <span class="lvm-card__title">{{ $gettext('Logical Volumes') }}</span>
                <n-tag size="small" :bordered="false">
                  {{ (lvmInfo.lvs || []).length }}
                </n-tag>
              </div>
              <div class="lvm-card__body">
                <div v-for="(lv, index) in lvmInfo.lvs || []" :key="index" class="lvm-item">
                  <div class="lvm-item__main">
                    <div class="lvm-item__name">{{ lv.field_0 }}</div>
                    <div class="lvm-item__meta">VG: {{ lv.field_1 }} · {{ lv.field_2 }}</div>
                    <div class="lvm-item__path">{{ lv.field_3 }}</div>
                  </div>
                  <n-button size="tiny" quaternary type="error" @click="handleRemoveLV(lv.field_3)">
                    <template #icon>
                      <i-mdi-close />
                    </template>
                  </n-button>
                </div>
                <n-empty
                  v-if="!(lvmInfo.lvs || []).length"
                  size="small"
                  :description="$gettext('No logical volumes')"
                />
              </div>
            </div>
          </n-gi>
        </n-grid>

        <!-- LVM 操作面板 -->
        <n-tabs
          v-model:value="lvmOpTab"
          type="line"
          animated
          size="small"
          :theme-overrides="{
            tabGapSmallLine: '20px',
            tabPaddingSmallLine: '8px 0',
          }"
          pane-style="padding-top: 16px;"
        >
          <n-tab-pane name="pv" :tab="$gettext('Create PV')">
            <n-flex vertical :size="12">
              <div>
                <div class="form-label">{{ $gettext('Device') }}</div>
                <n-select
                  v-model:value="pvDevice"
                  :options="availablePVDeviceOptions"
                  :placeholder="$gettext('Select device')"
                  filterable
                />
              </div>
              <n-flex justify="end">
                <n-button type="primary" @click="handleCreatePV">
                  {{ $gettext('Create PV') }}
                </n-button>
              </n-flex>
            </n-flex>
          </n-tab-pane>

          <n-tab-pane name="vg" :tab="$gettext('Create VG')">
            <n-flex vertical :size="12">
              <div>
                <div class="form-label">{{ $gettext('VG Name') }}</div>
                <n-input v-model:value="vgName" :placeholder="$gettext('Enter VG name')" />
              </div>
              <div>
                <div class="form-label">{{ $gettext('Physical Volumes') }}</div>
                <n-select
                  v-model:value="vgDevices"
                  :options="availablePVOptions"
                  :placeholder="$gettext('Select PVs')"
                  multiple
                  filterable
                />
              </div>
              <n-flex justify="end">
                <n-button type="primary" @click="handleCreateVG">
                  {{ $gettext('Create VG') }}
                </n-button>
              </n-flex>
            </n-flex>
          </n-tab-pane>

          <n-tab-pane name="lv" :tab="$gettext('Create LV')">
            <n-flex vertical :size="12">
              <n-grid :x-gap="12" :y-gap="12" cols="1 m:2" responsive="screen">
                <n-gi>
                  <div class="form-label">{{ $gettext('LV Name') }}</div>
                  <n-input v-model:value="lvName" :placeholder="$gettext('Enter LV name')" />
                </n-gi>
                <n-gi>
                  <div class="form-label">{{ $gettext('Volume Group') }}</div>
                  <n-select
                    v-model:value="lvVgName"
                    :options="vgOptions"
                    :placeholder="$gettext('Select VG')"
                    filterable
                  />
                </n-gi>
                <n-gi>
                  <div class="form-label">{{ $gettext('Size (GB)') }}</div>
                  <n-input-number v-model:value="lvSize" :min="1" class="!w-full" />
                </n-gi>
              </n-grid>
              <n-flex justify="end">
                <n-button type="primary" @click="handleCreateLV">
                  {{ $gettext('Create LV') }}
                </n-button>
              </n-flex>
            </n-flex>
          </n-tab-pane>

          <n-tab-pane name="extend" :tab="$gettext('Extend LV')">
            <n-flex vertical :size="12">
              <n-grid :x-gap="12" :y-gap="12" cols="1 m:2" responsive="screen">
                <n-gi>
                  <div class="form-label">{{ $gettext('Logical Volume') }}</div>
                  <n-select
                    v-model:value="extendLvPath"
                    :options="lvOptions"
                    :placeholder="$gettext('Select LV')"
                    filterable
                  />
                </n-gi>
                <n-gi>
                  <div class="form-label">{{ $gettext('Extend Size (GB)') }}</div>
                  <n-input-number v-model:value="extendSize" :min="1" class="!w-full" />
                </n-gi>
                <n-gi>
                  <div class="form-label">{{ $gettext('Auto Resize Filesystem') }}</div>
                  <n-switch v-model:value="extendResize" />
                </n-gi>
              </n-grid>
              <n-flex justify="end">
                <n-button type="primary" @click="handleExtendLV">
                  {{ $gettext('Extend LV') }}
                </n-button>
              </n-flex>
            </n-flex>
          </n-tab-pane>
        </n-tabs>
      </n-flex>
    </n-tab-pane>

    <n-tab-pane name="smart" tab="SMART">
      <smart-view />
    </n-tab-pane>

    <n-tab-pane name="raid" tab="RAID">
      <raid-view />
    </n-tab-pane>
  </n-tabs>
</template>

<style scoped lang="scss">
.disk-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.disk-card {
  padding: 16px;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border-default);
  border-radius: 3px;
}

.disk-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.disk-card__title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.disk-card__name {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.disk-card__meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--color-text-tertiary);
}

.disk-card__sep {
  opacity: 0.5;
}

.form-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-secondary);
  margin-bottom: 6px;
}

.lvm-card {
  display: flex;
  flex-direction: column;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border-default);
  border-radius: 3px;
  height: 100%;
  min-height: 200px;
}

.lvm-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.lvm-card__title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.lvm-card__body {
  flex: 1;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 280px;
  overflow-y: auto;
}

.lvm-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 3px;
  transition: background 150ms ease;

  &:hover {
    background: var(--color-bg-subtle);
  }
}

.lvm-item__main {
  flex: 1;
  min-width: 0;
}

.lvm-item__name {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-primary);
  font-family: 'JetBrains Mono Variable', monospace;
}

.lvm-item__meta {
  margin-top: 2px;
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.lvm-item__path {
  margin-top: 2px;
  font-size: 12px;
  color: var(--color-text-tertiary);
  font-family: 'JetBrains Mono Variable', monospace;
}
</style>

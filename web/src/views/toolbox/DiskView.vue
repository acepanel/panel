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
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useResponsive } from '@/components/system/composables/useResponsive'
import { formatBytes } from '@/utils'

import LvmView from './disk/LvmView.vue'
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
const { isMobile } = useResponsive()
const { confirmAction } = useConfirm()
const currentTab = ref('disk')
const diskOpTab = ref('mount')
const diskList = ref<DiskData[]>([])

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

const loadFstabList = () => {
  useRequest(disk.fstabList()).onSuccess(({ data }) => {
    fstabList.value = data || []
  })
}

onMounted(() => {
  loadDiskList()
  loadFstabList()
})

const handleMount = async () => {
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
  const ok = await confirmAction({ title: $gettext('Confirm'), content: confirmContent })
  if (!ok) return
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
}

const handleUmount = async (path: string) => {
  const ok = await confirmAction({
    title: $gettext('Confirm'),
    content: $gettext('Are you sure you want to unmount this partition?'),
  })
  if (!ok) return
  useRequest(disk.umount(path)).onSuccess(() => {
    window.$message.success($gettext('Unmounted successfully'))
    loadDiskList()
  })
}

const handleFormat = async () => {
  if (!formatDevice.value) {
    window.$message.error($gettext('Please select a device'))
    return
  }
  const ok = await confirmAction({
    type: 'error',
    title: $gettext('Dangerous Operation'),
    content: $gettext(
      'Formatting will erase all data on the partition. This operation is irreversible. Are you sure?',
    ),
    positiveText: $gettext('Confirm Format'),
  })
  if (!ok) return
  useRequest(disk.format(formatDevice.value, formatFsType.value)).onSuccess(() => {
    window.$message.success($gettext('Formatted successfully'))
    loadDiskList()
    formatDevice.value = ''
    formatFsType.value = 'ext4'
  })
}

const handleInit = async () => {
  if (!initDevice.value) {
    window.$message.error($gettext('Please enter disk name'))
    return
  }
  const ok = await confirmAction({
    type: 'error',
    title: $gettext('Dangerous Operation'),
    content: $gettext(
      'This will delete all partitions on %{ device } and create a single partition. All data will be permanently lost. Are you absolutely sure?',
      { device: initDevice.value },
    ),
    positiveText: $gettext('Confirm Initialize'),
  })
  if (!ok) return
  useRequest(disk.init(initDevice.value, initFsType.value)).onSuccess(() => {
    window.$message.success($gettext('Disk initialized successfully'))
    loadDiskList()
    initDevice.value = ''
    initFsType.value = 'ext4'
  })
}

const handleDeleteFstab = async (mountPoint: string) => {
  const ok = await confirmAction({
    type: 'error',
    title: $gettext('Dangerous Operation'),
    content: $gettext(
      'Are you sure you want to remove the fstab entry for %{ mountPoint }? This will prevent auto-mount on boot.',
      { mountPoint },
    ),
  })
  if (!ok) return
  useRequest(disk.fstabDelete(mountPoint)).onSuccess(() => {
    window.$message.success($gettext('Fstab entry removed successfully'))
    loadFstabList()
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
  <n-tabs v-model:value="currentTab" type="line" :placement="isMobile ? 'top' : 'left'" animated>
    <!-- 磁盘管理 -->
    <n-tab-pane name="disk" :tab="$gettext('Disk Management')">
      <n-flex vertical :size="16">
        <!-- 磁盘卡片列表 -->
        <div v-if="diskList.length > 0" class="disk-list">
          <div v-for="diskItem in diskList" :key="diskItem.name" class="disk-card">
            <div class="disk-card__head">
              <div class="disk-card__title">
                <i-mdi-harddisk class="text-xl text-brand" />
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
              :scroll-x="760"
              size="small"
              :row-key="(row: FstabEntry) => row.mount_point"
            />
            <n-empty v-else :description="$gettext('No fstab entries')" />
          </n-tab-pane>
        </n-tabs>
      </n-flex>
    </n-tab-pane>

    <n-tab-pane name="lvm" :tab="$gettext('LVM Management')">
      <lvm-view :disks="diskList" />
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
</style>

<script setup lang="ts">
import { useRequest } from 'alova/client'
import type { DataTableColumns } from 'naive-ui'
import { NTag } from 'naive-ui'
import { h } from 'vue'
import { useGettext } from 'vue3-gettext'

import disk from '@/api/panel/toolbox-disk'

const { $gettext } = useGettext()

interface RaidDevice {
  name: string
  slot: string
  size: string
  state: string
  model: string
  serial: string
}

interface RaidArray {
  name: string
  raid_level: string
  size: string
  state: string
  strip_size: string
  active_devices: number
  total_devices: number
  rebuild_pct: string
  devices: RaidDevice[]
}

interface RaidController {
  model: string
  serial: string
  firmware: string
  cache_size: string
}

const available = ref(false)
const unavailableMessage = ref('')
const raidType = ref('')
const controllers = ref<RaidController[]>([])
const arrays = ref<RaidArray[]>([])
const loading = ref(true)

const loadRaidInfo = () => {
  loading.value = true
  useRequest(disk.raidInfo()).onSuccess(({ data }) => {
    loading.value = false
    available.value = data.available
    unavailableMessage.value = data.message || ''
    raidType.value = data.type || ''
    controllers.value = data.controllers || []
    arrays.value = data.arrays || []
  })
}

onMounted(() => {
  loadRaidInfo()
})

// 阵列状态颜色
const getStateType = (state: string): 'success' | 'warning' | 'error' | 'info' => {
  if (!state) return 'info'
  const s = state.toLowerCase()
  if (s.includes('clean') || s.includes('active') || s.includes('optimal') || s === 'ok') {
    return 'success'
  }
  if (s.includes('degrad') || s.includes('rebuild') || s.includes('recover')) {
    return 'warning'
  }
  if (s.includes('fail') || s.includes('offline') || s.includes('error')) {
    return 'error'
  }
  return 'info'
}

// RAID 类型标签
const raidTypeLabel = computed(() => {
  const labels: Record<string, string> = {
    mdadm: 'Linux Software RAID (mdadm)',
    megaraid: 'MegaRAID (LSI/Broadcom)',
    hpsa: 'HP Smart Array',
    adaptec: 'Adaptec',
  }
  return labels[raidType.value] || raidType.value
})

// 物理磁盘表格列
const deviceColumns = computed<DataTableColumns<RaidDevice>>(() => {
  const cols: DataTableColumns<RaidDevice> = [
    { title: $gettext('Device'), key: 'name', width: 160 },
    { title: $gettext('Slot'), key: 'slot', width: 80 },
    { title: $gettext('Size'), key: 'size', width: 120 },
    {
      title: $gettext('Status'),
      key: 'state',
      width: 120,
      render(row) {
        return h(
          NTag,
          { type: getStateType(row.state), size: 'small' },
          { default: () => row.state || '-' },
        )
      },
    },
  ]
  // MegaRAID / HPSA / Adaptec 有 model 和 serial
  if (raidType.value !== 'mdadm') {
    cols.push({ title: $gettext('Model'), key: 'model', width: 160 })
    cols.push({ title: $gettext('Serial'), key: 'serial', width: 160 })
  }
  return cols
})
</script>

<template>
  <n-spin :show="loading">
    <!-- 不可用时 -->
    <n-empty
      v-if="!loading && !available"
      :description="unavailableMessage || $gettext('No RAID Detected')"
    />

    <!-- 可用时 -->
    <n-flex v-if="!loading && available" vertical :size="16">
      <!-- RAID 类型标识 -->
      <n-flex align="center" :size="8">
        <span>{{ $gettext('RAID Type') }}:</span>
        <n-tag type="info">{{ raidTypeLabel }}</n-tag>
      </n-flex>

      <!-- 控制器信息 -->
      <n-card
        v-for="(ctrl, i) in controllers"
        :key="i"
        :title="$gettext('Controller') + ` #${i + 1}`"
        size="small"
      >
        <n-descriptions bordered :column="2" label-placement="left" size="small">
          <n-descriptions-item v-if="ctrl.model" :label="$gettext('Model')">
            {{ ctrl.model }}
          </n-descriptions-item>
          <n-descriptions-item v-if="ctrl.serial" :label="$gettext('Serial Number')">
            {{ ctrl.serial }}
          </n-descriptions-item>
          <n-descriptions-item v-if="ctrl.firmware" :label="$gettext('Firmware')">
            {{ ctrl.firmware }}
          </n-descriptions-item>
          <n-descriptions-item v-if="ctrl.cache_size" :label="$gettext('Cache Size')">
            {{ ctrl.cache_size }}
          </n-descriptions-item>
        </n-descriptions>
      </n-card>

      <!-- 阵列信息 -->
      <n-card v-for="(arr, i) in arrays" :key="'arr-' + i" size="small">
        <template #header>
          <n-flex align="center" :size="8">
            <span>{{ arr.name }}</span>
            <n-tag :type="getStateType(arr.state)" size="small">{{ arr.state || '-' }}</n-tag>
          </n-flex>
        </template>
        <template #header-extra>
          <n-flex align="center" :size="16">
            <span v-if="arr.raid_level">{{ arr.raid_level }}</span>
            <span v-if="arr.size">{{ arr.size }}</span>
          </n-flex>
        </template>

        <n-flex vertical :size="12">
          <n-descriptions bordered :column="3" label-placement="left" size="small">
            <n-descriptions-item v-if="arr.raid_level" :label="$gettext('RAID Level')">
              {{ arr.raid_level }}
            </n-descriptions-item>
            <n-descriptions-item v-if="arr.size" :label="$gettext('Size')">
              {{ arr.size }}
            </n-descriptions-item>
            <n-descriptions-item v-if="arr.strip_size" :label="$gettext('Strip Size')">
              {{ arr.strip_size }}
            </n-descriptions-item>
            <n-descriptions-item
              v-if="arr.active_devices || arr.total_devices"
              :label="$gettext('Devices')"
            >
              {{ arr.active_devices }} / {{ arr.total_devices }}
            </n-descriptions-item>
            <n-descriptions-item v-if="arr.rebuild_pct" :label="$gettext('Rebuild Progress')">
              <n-tag type="warning" size="small">{{ arr.rebuild_pct }}</n-tag>
            </n-descriptions-item>
          </n-descriptions>

          <!-- 物理磁盘列表 -->
          <n-data-table
            v-if="arr.devices && arr.devices.length > 0"
            :columns="deviceColumns"
            :data="arr.devices"
            :bordered="false"
            :single-line="false"
            size="small"
            :row-key="(row: RaidDevice) => row.name + row.slot"
          />
        </n-flex>
      </n-card>

      <n-empty v-if="arrays.length === 0" :description="$gettext('No RAID arrays found')" />
    </n-flex>
  </n-spin>
</template>

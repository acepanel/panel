<script setup lang="ts">
import { useRequest } from 'alova/client'
import type { DataTableColumns } from 'naive-ui'
import { NTag } from 'naive-ui'
import { h } from 'vue'
import { useGettext } from 'vue3-gettext'

import disk from '@/api/panel/toolbox-disk'

const { $gettext } = useGettext()

// SMART 磁盘列表
interface SmartDisk {
  name: string
  model: string
  type: string
}

const available = ref(false)
const unavailableMessage = ref('')
const diskOptions = ref<{ label: string; value: string }[]>([])
const selectedDisk = ref('')
const smartData = ref<any>(null)
const loadingDisks = ref(true)
const loadingInfo = ref(false)

// 加载 SMART 磁盘列表
const loadSmartDisks = () => {
  loadingDisks.value = true
  useRequest(disk.smartDisks()).onSuccess(({ data }) => {
    loadingDisks.value = false
    available.value = data.available
    unavailableMessage.value = data.message || ''
    if (data.available && data.disks) {
      diskOptions.value = data.disks.map((d: SmartDisk) => ({
        label: d.model ? `${d.name} (${d.model})` : d.name,
        value: d.name,
      }))
      // 自动选中第一个
      if (diskOptions.value.length > 0) {
        selectedDisk.value = diskOptions.value[0]!.value
      }
    }
  })
}

// 加载 SMART 详情
const loadSmartInfo = () => {
  if (!selectedDisk.value) return
  loadingInfo.value = true
  smartData.value = null
  useRequest(disk.smartInfo(selectedDisk.value)).onSuccess(({ data }) => {
    loadingInfo.value = false
    smartData.value = data
  })
}

// 监听磁盘选择变化
watch(selectedDisk, () => {
  if (selectedDisk.value) {
    loadSmartInfo()
  }
})

onMounted(() => {
  loadSmartDisks()
})

// 提取温度
const temperature = computed(() => {
  if (!smartData.value) return null
  // ATA
  if (smartData.value.temperature?.current != null) {
    return smartData.value.temperature.current
  }
  // NVMe
  if (smartData.value.nvme_smart_health_information_log?.temperature != null) {
    return smartData.value.nvme_smart_health_information_log.temperature
  }
  return null
})

// 温度颜色
const temperatureColor = computed(() => {
  const temp = temperature.value
  if (temp == null) return '#18a058'
  if (temp <= 40) return '#18a058'
  if (temp <= 50) return '#f0a020'
  return '#d03050'
})

// 提取健康状态
const healthStatus = computed(() => {
  if (!smartData.value?.smart_status) return null
  return smartData.value.smart_status.passed
})

// 提取设备信息
const deviceInfo = computed(() => {
  if (!smartData.value) return []
  const d = smartData.value
  const items: { label: string; value: string }[] = []

  if (d.model_name) items.push({ label: $gettext('Model'), value: d.model_name })
  if (d.serial_number) items.push({ label: $gettext('Serial Number'), value: d.serial_number })
  if (d.firmware_version) items.push({ label: $gettext('Firmware'), value: d.firmware_version })
  if (d.user_capacity?.bytes) {
    items.push({
      label: $gettext('Capacity'),
      value: formatCapacity(d.user_capacity.bytes),
    })
  }
  if (d.device_type?.name) {
    items.push({ label: $gettext('Interface'), value: d.device_type.name })
  }
  if (d.rotation_rate != null) {
    items.push({
      label: $gettext('Rotation Rate'),
      value: d.rotation_rate === 0 ? 'SSD' : `${d.rotation_rate} RPM`,
    })
  }
  if (d.power_on_time?.hours != null) {
    items.push({ label: $gettext('Power On Hours'), value: `${d.power_on_time.hours} h` })
  }
  if (d.power_cycle_count != null) {
    items.push({ label: $gettext('Power Cycle Count'), value: `${d.power_cycle_count}` })
  }

  // NVMe 特有信息
  const nvme = d.nvme_smart_health_information_log
  if (nvme) {
    if (nvme.percentage_used != null) {
      items.push({ label: $gettext('Percentage Used'), value: `${nvme.percentage_used}%` })
    }
    if (nvme.data_units_read != null) {
      items.push({
        label: $gettext('Data Read'),
        value: formatCapacity(nvme.data_units_read * 512000),
      })
    }
    if (nvme.data_units_written != null) {
      items.push({
        label: $gettext('Data Written'),
        value: formatCapacity(nvme.data_units_written * 512000),
      })
    }
  }

  return items
})

// ATA SMART 属性表格
const ataAttributes = computed(() => {
  if (!smartData.value?.ata_smart_attributes?.table) return []
  return smartData.value.ata_smart_attributes.table
})

// NVMe SMART 信息
const nvmeAttributes = computed(() => {
  const nvme = smartData.value?.nvme_smart_health_information_log
  if (!nvme) return []
  const items: { key: string; name: string; value: string }[] = []
  const mapping: Record<string, string> = {
    critical_warning: $gettext('Critical Warning'),
    temperature: $gettext('Temperature'),
    available_spare: $gettext('Available Spare'),
    available_spare_threshold: $gettext('Available Spare Threshold'),
    percentage_used: $gettext('Percentage Used'),
    data_units_read: $gettext('Data Units Read'),
    data_units_written: $gettext('Data Units Written'),
    host_reads: $gettext('Host Read Commands'),
    host_writes: $gettext('Host Write Commands'),
    controller_busy_time: $gettext('Controller Busy Time'),
    power_cycles: $gettext('Power Cycles'),
    power_on_hours: $gettext('Power On Hours'),
    unsafe_shutdowns: $gettext('Unsafe Shutdowns'),
    media_errors: $gettext('Media Errors'),
    num_err_log_entries: $gettext('Error Log Entries'),
  }
  for (const [key, label] of Object.entries(mapping)) {
    if (nvme[key] != null) {
      let val = String(nvme[key])
      if (key === 'temperature') val += ' °C'
      else if (
        key === 'available_spare' ||
        key === 'available_spare_threshold' ||
        key === 'percentage_used'
      )
        val += '%'
      items.push({ key, name: label, value: val })
    }
  }
  return items
})

// 是否为 NVMe 设备
const isNVMe = computed(() => {
  return !!smartData.value?.nvme_smart_health_information_log
})

// ATA 属性表格列
const ataColumns = computed<DataTableColumns>(() => [
  { title: 'ID', key: 'id', width: 60 },
  { title: $gettext('Attribute'), key: 'name', width: 220 },
  { title: $gettext('Value'), key: 'value', width: 80 },
  { title: $gettext('Worst'), key: 'worst', width: 80 },
  { title: $gettext('Threshold'), key: 'thresh', width: 80 },
  {
    title: $gettext('Raw Value'),
    key: 'raw',
    width: 150,
    render(row: any) {
      return String(row.raw?.value ?? '')
    },
  },
  {
    title: $gettext('Status'),
    key: 'when_failed',
    width: 100,
    render(row: any) {
      if (row.when_failed && row.when_failed !== '') {
        return h(NTag, { type: 'error', size: 'small' }, { default: () => row.when_failed })
      }
      return h(NTag, { type: 'success', size: 'small' }, { default: () => 'OK' })
    },
  },
])

// NVMe 属性表格列
const nvmeColumns = computed<DataTableColumns>(() => [
  { title: $gettext('Attribute'), key: 'name', width: 220 },
  { title: $gettext('Value'), key: 'value' },
])

// 格式化容量
const formatCapacity = (bytes: number): string => {
  if (bytes < 1024) return bytes + ' B'
  const units = ['KB', 'MB', 'GB', 'TB', 'PB']
  let i = -1
  let val = bytes
  do {
    val /= 1024
    i++
  } while (val >= 1024 && i < units.length - 1)
  return val.toFixed(2) + ' ' + units[i]
}
</script>

<template>
  <n-spin :show="loadingDisks">
    <!-- 不可用时 -->
    <n-empty
      v-if="!loadingDisks && !available"
      :description="unavailableMessage || $gettext('SMART Not Available')"
    />

    <!-- 可用时 -->
    <n-flex v-if="!loadingDisks && available" vertical :size="16">
      <!-- 磁盘选择 -->
      <n-flex v-if="diskOptions.length > 0" align="center" :size="12">
        <span>{{ $gettext('Select Disk') }}:</span>
        <n-select
          v-model:value="selectedDisk"
          :options="diskOptions"
          class="w-75"
          :placeholder="$gettext('Select a disk')"
        />
      </n-flex>

      <!-- 无磁盘 -->
      <n-empty
        v-if="diskOptions.length === 0"
        :description="$gettext('No SMART-capable disks found')"
      />

      <!-- SMART 数据 -->
      <n-spin v-if="selectedDisk" :show="loadingInfo">
        <n-tabs v-if="smartData" type="line" animated>
          <!-- 基本信息 -->
          <n-tab-pane name="info" :tab="$gettext('Basic Info')">
            <n-flex :size="24">
              <!-- 温度圆形进度条 -->
              <n-flex v-if="temperature != null" vertical align="center" :size="8">
                <n-progress
                  type="circle"
                  :percentage="Math.min(temperature, 100)"
                  :color="temperatureColor"
                  :rail-color="temperatureColor + '20'"
                  :stroke-width="10"
                  class="w-30"
                >
                  <span>{{ temperature }}°C</span>
                </n-progress>
                <span style="color: var(--text-color-3)">{{ $gettext('Temperature') }}</span>
              </n-flex>

              <!-- 设备详情 -->
              <n-flex vertical :size="12" style="flex: 1">
                <!-- 健康状态 -->
                <n-flex v-if="healthStatus != null" align="center" :size="8">
                  <span>{{ $gettext('Health Status') }}:</span>
                  <n-tag :type="healthStatus ? 'success' : 'error'" size="small">
                    {{ healthStatus ? $gettext('PASSED') : $gettext('FAILED') }}
                  </n-tag>
                </n-flex>

                <n-descriptions bordered :column="2" label-placement="left" size="small">
                  <n-descriptions-item
                    v-for="item in deviceInfo"
                    :key="item.label"
                    :label="item.label"
                  >
                    {{ item.value }}
                  </n-descriptions-item>
                </n-descriptions>
              </n-flex>
            </n-flex>
          </n-tab-pane>

          <!-- SMART 属性 -->
          <n-tab-pane name="attributes" :tab="$gettext('SMART Attributes')">
            <!-- ATA 设备 -->
            <n-data-table
              v-if="!isNVMe && ataAttributes.length > 0"
              :columns="ataColumns"
              :data="ataAttributes"
              :bordered="false"
              :single-line="false"
              size="small"
              :row-key="(row: any) => row.id"
            />
            <!-- NVMe 设备 -->
            <n-data-table
              v-else-if="isNVMe && nvmeAttributes.length > 0"
              :columns="nvmeColumns"
              :data="nvmeAttributes"
              :bordered="false"
              :single-line="false"
              size="small"
              :row-key="(row: any) => row.key"
            />
            <n-empty v-else :description="$gettext('No SMART attributes available')" />
          </n-tab-pane>
        </n-tabs>
      </n-spin>
    </n-flex>
  </n-spin>
</template>

<script setup lang="ts">
import { useRequest } from 'alova/client'
import { useGettext } from 'vue3-gettext'

import disk from '@/api/panel/toolbox-disk'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { formatBytes } from '@/utils'

interface Disk {
  name: string
  size: number
  isSystemDisk: boolean
  partitions: { name: string; size: number; mountpoint: string | null }[]
}

interface OverviewItem {
  name: string
  meta: string
  path?: string
  remove: () => void
}

const props = defineProps<{
  disks: Disk[]
}>()

const { $gettext } = useGettext()
const { confirmAction } = useConfirm()
const lvmOpTab = ref('pv')
const lvmInfo = ref<any>({ pvs: [], vgs: [], lvs: [] })

const pvDevice = ref('')
const vgName = ref('')
const vgDevices = ref<string[]>([])
const lvName = ref('')
const lvVgName = ref('')
const lvSize = ref(1)
const extendLvPath = ref('')
const extendSize = ref(1)
const extendResize = ref(true)

const availablePVDeviceOptions = computed(() => {
  const options: { label: string; value: string }[] = []
  const existingPVs = new Set(lvmInfo.value.pvs?.map((pv: any) => pv.field_0) || [])
  for (const d of props.disks) {
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
  const free = $gettext('Free')
  return (lvmInfo.value.vgs || []).map((vg: any) => ({
    label: `${vg.field_0} (${free}: ${vg.field_4})`,
    value: vg.field_0,
  }))
})

const lvOptions = computed(() => {
  return (lvmInfo.value.lvs || []).map((lv: any) => ({
    label: `${lv.field_0} (${lv.field_2}) - ${lv.field_3}`,
    value: lv.field_3,
  }))
})

// PV / VG / LV 三张概览卡结构相同，只有文案和每行内容不同
const overview = computed<{ title: string; empty: string; items: OverviewItem[] }[]>(() => {
  const free = $gettext('Free')
  return [
    {
      title: $gettext('Physical Volumes'),
      empty: $gettext('No physical volumes'),
      items: (lvmInfo.value.pvs || []).map((pv: any) => ({
        name: pv.field_0,
        meta:
          `VG: ${pv.field_1 || '-'} · ${pv.field_2}` +
          (pv.field_3 ? ` · ${free}: ${pv.field_3}` : ''),
        remove: () => handleRemovePV(pv.field_0),
      })),
    },
    {
      title: $gettext('Volume Groups'),
      empty: $gettext('No volume groups'),
      items: (lvmInfo.value.vgs || []).map((vg: any) => ({
        name: vg.field_0,
        meta: `PV: ${vg.field_1} · LV: ${vg.field_2} · ${vg.field_3} · ${free}: ${vg.field_4}`,
        remove: () => handleRemoveVG(vg.field_0),
      })),
    },
    {
      title: $gettext('Logical Volumes'),
      empty: $gettext('No logical volumes'),
      items: (lvmInfo.value.lvs || []).map((lv: any) => ({
        name: lv.field_0,
        meta: `VG: ${lv.field_1} · ${lv.field_2}`,
        path: lv.field_3,
        remove: () => handleRemoveLV(lv.field_3),
      })),
    },
  ]
})

const loadLVMInfo = () => {
  useRequest(disk.lvmInfo()).onSuccess(({ data }) => {
    lvmInfo.value = data
  })
}

onMounted(() => {
  loadLVMInfo()
})

const handleCreatePV = async () => {
  if (!pvDevice.value) {
    window.$message.error($gettext('Please select a device'))
    return
  }
  const ok = await confirmAction({
    title: $gettext('Confirm'),
    content: $gettext('Are you sure you want to create a physical volume on %{ device }?', {
      device: pvDevice.value,
    }),
  })
  if (!ok) return
  useRequest(disk.createPV(pvDevice.value)).onSuccess(() => {
    window.$message.success($gettext('Physical volume created successfully'))
    loadLVMInfo()
    pvDevice.value = ''
  })
}

const handleRemovePV = async (device: string) => {
  const ok = await confirmAction({
    type: 'error',
    title: $gettext('Dangerous Operation'),
    content: $gettext('Are you sure you want to remove the physical volume %{ device }?', {
      device,
    }),
  })
  if (!ok) return
  useRequest(disk.removePV(device)).onSuccess(() => {
    window.$message.success($gettext('Physical volume removed successfully'))
    loadLVMInfo()
  })
}

const handleCreateVG = async () => {
  if (!vgName.value || vgDevices.value.length === 0) {
    window.$message.error($gettext('Please fill in all fields'))
    return
  }
  const ok = await confirmAction({
    title: $gettext('Confirm'),
    content: $gettext('Are you sure you want to create volume group %{ name }?', {
      name: vgName.value,
    }),
  })
  if (!ok) return
  useRequest(disk.createVG(vgName.value, vgDevices.value)).onSuccess(() => {
    window.$message.success($gettext('Volume group created successfully'))
    loadLVMInfo()
    vgName.value = ''
    vgDevices.value = []
  })
}

const handleRemoveVG = async (name: string) => {
  const ok = await confirmAction({
    type: 'error',
    title: $gettext('Dangerous Operation'),
    content: $gettext(
      'Are you sure you want to remove the volume group %{ name }? All logical volumes in this group will be deleted!',
      { name },
    ),
  })
  if (!ok) return
  useRequest(disk.removeVG(name)).onSuccess(() => {
    window.$message.success($gettext('Volume group removed successfully'))
    loadLVMInfo()
  })
}

const handleCreateLV = async () => {
  if (!lvName.value || !lvVgName.value || lvSize.value < 1) {
    window.$message.error($gettext('Please fill in all fields'))
    return
  }
  const ok = await confirmAction({
    title: $gettext('Confirm'),
    content: $gettext(
      'Are you sure you want to create logical volume %{ name } with %{ size }GB?',
      { name: lvName.value, size: lvSize.value },
    ),
  })
  if (!ok) return
  useRequest(disk.createLV(lvName.value, lvVgName.value, lvSize.value)).onSuccess(() => {
    window.$message.success($gettext('Logical volume created successfully'))
    loadLVMInfo()
    lvName.value = ''
    lvVgName.value = ''
    lvSize.value = 1
  })
}

const handleRemoveLV = async (path: string) => {
  const ok = await confirmAction({
    type: 'error',
    title: $gettext('Dangerous Operation'),
    content: $gettext(
      'Are you sure you want to remove the logical volume %{ path }? All data on this volume will be lost!',
      { path },
    ),
  })
  if (!ok) return
  useRequest(disk.removeLV(path)).onSuccess(() => {
    window.$message.success($gettext('Logical volume removed successfully'))
    loadLVMInfo()
  })
}

const handleExtendLV = async () => {
  if (!extendLvPath.value || extendSize.value < 1) {
    window.$message.error($gettext('Please fill in all fields'))
    return
  }
  const ok = await confirmAction({
    title: $gettext('Confirm'),
    content: $gettext('Are you sure you want to extend %{ path } by %{ size }GB?', {
      path: extendLvPath.value,
      size: extendSize.value,
    }),
  })
  if (!ok) return
  useRequest(disk.extendLV(extendLvPath.value, extendSize.value, extendResize.value)).onSuccess(
    () => {
      window.$message.success($gettext('Logical volume extended successfully'))
      loadLVMInfo()
      extendLvPath.value = ''
      extendSize.value = 1
    },
  )
}
</script>

<template>
  <n-flex vertical :size="16">
    <!-- PV / VG / LV 概览 -->
    <n-grid :x-gap="16" :y-gap="16" cols="1 l:3" responsive="screen">
      <n-gi v-for="card in overview" :key="card.title">
        <div class="lvm-card">
          <div class="lvm-card__head">
            <span class="lvm-card__title">{{ card.title }}</span>
            <n-tag size="small" :bordered="false">
              {{ card.items.length }}
            </n-tag>
          </div>
          <div class="lvm-card__body">
            <div v-for="(item, index) in card.items" :key="index" class="lvm-item">
              <div class="lvm-item__main">
                <div class="lvm-item__name">{{ item.name }}</div>
                <div class="lvm-item__meta">{{ item.meta }}</div>
                <div v-if="item.path" class="lvm-item__path">{{ item.path }}</div>
              </div>
              <n-button size="tiny" quaternary type="error" @click="item.remove">
                <template #icon>
                  <i-mdi-close />
                </template>
              </n-button>
            </div>
            <n-empty v-if="!card.items.length" size="small" :description="card.empty" />
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
</template>

<style scoped lang="scss">
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

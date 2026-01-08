<script setup lang="ts">
defineOptions({
  name: 'toolbox-disk'
})

import { useRequest } from 'alova/client'
import { onMounted, ref } from 'vue'
import { useGettext } from 'vue3-gettext'

import disk from '@/api/panel/toolbox-disk'

const { $gettext } = useGettext()
const currentTab = ref('disk')
const diskList = ref<any>({})
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

// 加载磁盘列表
const loadDiskList = () => {
  useRequest(disk.list()).onSuccess(({ data }) => {
    try {
      diskList.value = JSON.parse(data)
    } catch (e) {
      diskList.value = {}
      console.error('解析磁盘列表数据失败:', e)
      window.$message.error($gettext('Failed to parse disk data, please refresh and try again'))
    }
  })
}

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
      <n-flex vertical>
        <n-card :title="$gettext('Disk List')">
          <n-space vertical>
            <n-button @click="loadDiskList">{{ $gettext('Refresh') }}</n-button>
            <pre>{{ JSON.stringify(diskList, null, 2) }}</pre>
          </n-space>
        </n-card>

        <n-card :title="$gettext('Mount Partition')">
          <n-form>
            <n-form-item :label="$gettext('Device')">
              <n-input v-model:value="selectedDevice" :placeholder="$gettext('e.g., sdb1')" />
            </n-form-item>
            <n-form-item :label="$gettext('Mount Path')">
              <n-input v-model:value="mountPath" :placeholder="$gettext('e.g., /mnt/data')" />
            </n-form-item>
            <n-button type="primary" @click="handleMount">{{ $gettext('Mount') }}</n-button>
          </n-form>
        </n-card>

        <n-card :title="$gettext('Format Partition')">
          <n-alert type="error" style="margin-bottom: 16px">
            {{ $gettext('Warning: Formatting will erase all data!') }}
          </n-alert>
          <n-form>
            <n-form-item :label="$gettext('Device')">
              <n-input v-model:value="formatDevice" :placeholder="$gettext('e.g., sdb1')" />
            </n-form-item>
            <n-form-item :label="$gettext('Filesystem Type')">
              <n-select v-model:value="formatFsType" :options="fsTypeOptions" />
            </n-form-item>
            <n-button type="error" @click="handleFormat">{{ $gettext('Format') }}</n-button>
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

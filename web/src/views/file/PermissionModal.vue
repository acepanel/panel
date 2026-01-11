<script setup lang="ts">
import { NButton, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import type { FileInfo } from '@/views/file/types'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const selected = defineModel<string[]>('selected', { type: Array, required: true })
// 文件信息列表，用于获取当前所有者和组
const fileInfoList = defineModel<FileInfo[]>('fileInfoList', { type: Array, default: () => [] })
const mode = ref('755')
const owner = ref('www')
const group = ref('www')

const checkbox = ref({
  owner: ['read', 'write', 'execute'],
  group: ['read', 'execute'],
  other: ['read', 'execute']
})

// 规范化 mode 字符串，确保为3位数字
const normalizeMode = (modeStr: string): string => {
  // 去掉前导0，但保留至少一位数字
  const trimmed = modeStr.replace(/^0+(?=\d)/, '')
  // 确保 mode 至少有3位，不足则左补0
  return trimmed.padStart(3, '0') || '755'
}

// 当打开弹窗时，从文件信息中获取当前权限/所有者/组
watch(
  () => show.value,
  (newVal) => {
    if (newVal && fileInfoList.value.length > 0) {
      const firstFile = fileInfoList.value[0]
      mode.value = normalizeMode(firstFile.mode)
      owner.value = firstFile.owner || 'www'
      group.value = firstFile.group || 'www'
      updateCheckboxes()
    }
  }
)

const handlePermission = async () => {
  const promises = selected.value.map((path) =>
    file.permission(path, `0${mode.value}`, owner.value, group.value)
  )
  await Promise.all(promises)

  show.value = false
  selected.value = []
  fileInfoList.value = []
  window.$bus.emit('file:refresh')
  window.$message.success($gettext('Modified successfully'))
}

const calculateOctal = (permissions: string[]) => {
  let octal = 0
  if (permissions.includes('read')) octal += 4
  if (permissions.includes('write')) octal += 2
  if (permissions.includes('execute')) octal += 1
  return octal
}

const calculateMode = () => {
  const owner = calculateOctal(checkbox.value.owner)
  const group = calculateOctal(checkbox.value.group)
  const other = calculateOctal(checkbox.value.other)

  mode.value = `${owner}${group}${other}`
}

const updateCheckboxes = () => {
  const paddedMode = normalizeMode(mode.value)
  const permissions = paddedMode.split('').map(Number)

  checkbox.value.owner = permissions[0] & 4 ? ['read'] : []
  if (permissions[0] & 2) checkbox.value.owner.push('write')
  if (permissions[0] & 1) checkbox.value.owner.push('execute')

  checkbox.value.group = permissions[1] & 4 ? ['read'] : []
  if (permissions[1] & 2) checkbox.value.group.push('write')
  if (permissions[1] & 1) checkbox.value.group.push('execute')

  checkbox.value.other = permissions[2] & 4 ? ['read'] : []
  if (permissions[2] & 2) checkbox.value.other.push('write')
  if (permissions[2] & 1) checkbox.value.other.push('execute')
}

const title = computed(() => {
  if (selected.value.length === 0) {
    return $gettext('Modify permissions')
  }
  return selected.value.length > 1
    ? $gettext('Batch modify permissions')
    : $gettext('Modify permissions - %{ path }', { path: selected.value[0] })
})

watch(mode, updateCheckboxes, { immediate: true })
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="title"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-flex vertical>
      <n-form>
        <n-row :gutter="[0, 24]">
          <n-col :span="8">
            <n-form-item :label="$gettext('Owner')">
              <n-checkbox-group v-model:value="checkbox.owner" @update:value="calculateMode">
                <n-checkbox value="read" :label="$gettext('Read')" />
                <n-checkbox value="write" :label="$gettext('Write')" />
                <n-checkbox value="execute" :label="$gettext('Execute')" />
              </n-checkbox-group>
            </n-form-item>
          </n-col>
          <n-col :span="8">
            <n-form-item :label="$gettext('Group')">
              <n-checkbox-group v-model:value="checkbox.group" @update:value="calculateMode">
                <n-checkbox value="read" :label="$gettext('Read')" />
                <n-checkbox value="write" :label="$gettext('Write')" />
                <n-checkbox value="execute" :label="$gettext('Execute')" />
              </n-checkbox-group>
            </n-form-item>
          </n-col>
          <n-col :span="8">
            <n-form-item :label="$gettext('Others')">
              <n-checkbox-group v-model:value="checkbox.other" @update:value="calculateMode">
                <n-checkbox value="read" :label="$gettext('Read')" />
                <n-checkbox value="write" :label="$gettext('Write')" />
                <n-checkbox value="execute" :label="$gettext('Execute')" />
              </n-checkbox-group>
            </n-form-item>
          </n-col>
        </n-row>
        <n-form-item :label="$gettext('Permission')">
          <n-input v-model:value="mode" />
        </n-form-item>
        <n-form-item :label="$gettext('Owner')">
          <n-input v-model:value="owner" />
        </n-form-item>
        <n-form-item :label="$gettext('Group')">
          <n-input v-model:value="group" />
        </n-form-item>
      </n-form>
      <n-button type="primary" @click="handlePermission"> {{ $gettext('Modify') }} </n-button>
    </n-flex>
  </n-modal>
</template>

<style scoped lang="scss"></style>

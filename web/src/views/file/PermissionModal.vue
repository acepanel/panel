<script setup lang="ts">
import { NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import { useFileStore } from '@/stores'

const { $gettext } = useGettext()
const fileStore = useFileStore()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const selected = computed(() => fileStore.activeSelected)
const mode = ref('755')
const owner = ref('www')
const group = ref('www')
const recursive = ref(false)

const checkbox = ref({
  owner: ['read', 'write', 'execute'],
  group: ['read', 'execute'],
  other: ['read', 'execute'],
})

// 规范化 mode 字符串，确保为3位数字
const normalizeMode = (modeStr: string): string => {
  // 去掉前导0，但保留至少一位数字
  const trimmed = modeStr.replace(/^0+(?=\d)/, '')
  // 确保 mode 至少有3位，不足则左补0
  return trimmed.padStart(3, '0') || '755'
}

// 初值返回前表单还是上次的值，期间遮住表单，避免按旧值提交
const infoLoading = ref(false)

// 打开弹窗时拉取第一个选中文件的信息作为初值，避免依赖调用方传入导致残留旧值
watch(
  () => show.value,
  (newVal) => {
    if (!newVal || !selected.value.length) return
    recursive.value = false
    infoLoading.value = true
    useRequest(file.info(selected.value[0]!))
      .onSuccess(({ data }) => {
        mode.value = normalizeMode(data.mode)
        owner.value = data.owner || 'www'
        group.value = data.group || 'www'
        updateCheckboxes()
      })
      .onComplete(() => {
        infoLoading.value = false
      })
  },
)

const handlePermission = async () => {
  // allSettled 保证部分失败时也刷新列表，失败的错误弹窗由全局拦截器负责
  const results = await Promise.allSettled(
    selected.value.map((path) =>
      file.permission(path, `0${mode.value}`, owner.value, group.value, recursive.value),
    ),
  )

  window.$bus.emit('file:refresh')
  if (results.some((r) => r.status === 'rejected')) return

  show.value = false
  fileStore.selected[fileStore.activeTabId] = []
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
  const permissions = paddedMode.split('').map(Number) as [number, number, number]

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
    : $gettext('Modify permissions - %{ path }', { path: selected.value[0] ?? '' })
})

watch(mode, updateCheckboxes, { immediate: true })
</script>

<template>
  <app-modal v-model:show="show" :title="title">
    <n-spin :show="infoLoading">
      <n-flex vertical>
        <n-form>
          <n-grid cols="1 m:24" responsive="screen">
            <n-form-item-gi :span="8" :label="$gettext('Owner')">
              <n-checkbox-group v-model:value="checkbox.owner" @update:value="calculateMode">
                <n-checkbox value="read" :label="$gettext('Read')" />
                <n-checkbox value="write" :label="$gettext('Write')" />
                <n-checkbox value="execute" :label="$gettext('Execute')" />
              </n-checkbox-group>
            </n-form-item-gi>
            <n-form-item-gi :span="8" :label="$gettext('Group')">
              <n-checkbox-group v-model:value="checkbox.group" @update:value="calculateMode">
                <n-checkbox value="read" :label="$gettext('Read')" />
                <n-checkbox value="write" :label="$gettext('Write')" />
                <n-checkbox value="execute" :label="$gettext('Execute')" />
              </n-checkbox-group>
            </n-form-item-gi>
            <n-form-item-gi :span="8" :label="$gettext('Others')">
              <n-checkbox-group v-model:value="checkbox.other" @update:value="calculateMode">
                <n-checkbox value="read" :label="$gettext('Read')" />
                <n-checkbox value="write" :label="$gettext('Write')" />
                <n-checkbox value="execute" :label="$gettext('Execute')" />
              </n-checkbox-group>
            </n-form-item-gi>
          </n-grid>
          <n-form-item :label="$gettext('Permission')">
            <n-input v-model:value="mode" />
          </n-form-item>
          <n-form-item :label="$gettext('Owner')">
            <n-input v-model:value="owner" />
          </n-form-item>
          <n-form-item :label="$gettext('Group')">
            <n-input v-model:value="group" />
          </n-form-item>
          <n-form-item :show-label="false">
            <n-checkbox v-model:checked="recursive">
              {{ $gettext('Apply to all files and subdirectories') }}
            </n-checkbox>
          </n-form-item>
        </n-form>
        <app-button type="primary" @click="handlePermission"> {{ $gettext('Modify') }} </app-button>
      </n-flex>
    </n-spin>
  </app-modal>
</template>

<style scoped lang="scss"></style>

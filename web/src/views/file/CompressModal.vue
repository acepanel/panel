<script setup lang="ts">
import { NButton, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import api from '@/api/panel/file'
import { generateRandomString, getBase } from '@/utils'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const path = defineModel<string>('path', { type: String, required: true })
const selected = defineModel<string[]>('selected', { type: Array, default: () => [] })
const format = ref('.zip')
const loading = ref(false)

// 生成随机文件名
const generateName = () => {
  // 如果选择多个文件，文件名为目录名 + 随机字符串，否则就直接文件名 + 随机字符串
  // 特殊处理根目录，防止出现 //xxx 的情况
  if (path.value == '/') {
    return selected.value.length > 1
      ? `/${generateRandomString(6)}${format.value}`
      : `${selected.value[0]}-${generateRandomString(6)}${format.value}`
  }
  const parts = path.value.split('/')
  return selected.value.length > 1
    ? `${path.value}/${parts.pop()}-${generateRandomString(6)}${format.value}`
    : `${selected.value[0]}-${generateRandomString(6)}${format.value}`
}

const file = ref(generateName())

const ensureExtension = (extension: string) => {
  if (!file.value.endsWith(extension)) {
    file.value = `${getBase(file.value)}${extension}`
  }
}

const handleArchive = () => {
  ensureExtension(format.value)
  loading.value = true
  const message = window.$message.loading($gettext('Compressing...'), {
    duration: 0
  })
  const paths = selected.value.map((item) => item.replace(path.value, '').replace(/^\//, ''))
  useRequest(api.compress(path.value, paths, file.value))
    .onSuccess(() => {
      show.value = false
      selected.value = []
      window.$message.success($gettext('Compressed successfully'))
    })
    .onComplete(() => {
      message?.destroy()
      loading.value = false
      window.$bus.emit('file:refresh')
    })
}

onMounted(() => {
  watch(
    selected,
    () => {
      file.value = generateName()
    },
    { immediate: true }
  )
})
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$gettext('Compress')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-flex vertical>
      <n-form>
        <n-form-item :label="$gettext('Files to compress')">
          <n-dynamic-input v-model:value="selected" :min="1" />
        </n-form-item>
        <n-form-item :label="$gettext('Compress to')">
          <n-input v-model:value="file" />
        </n-form-item>
        <n-form-item :label="$gettext('Format')">
          <n-select
            v-model:value="format"
            :options="[
              { label: '.zip', value: '.zip' },
              { label: '.gz', value: '.gz' },
              { label: '.tar', value: '.tar' },
              { label: '.tar.gz', value: '.tar.gz' },
              { label: '.tgz', value: '.tgz' },
              { label: '.tar.bz2', value: '.tar.bz2' },
              { label: '.tar.xz', value: '.tar.xz' },
              { label: '.7z', value: '.7z' }
            ]"
            @update:value="ensureExtension"
          />
        </n-form-item>
      </n-form>
      <n-button :loading="loading" :disabled="loading" type="primary" @click="handleArchive">
        {{ $gettext('Compress') }}
      </n-button>
    </n-flex>
  </n-modal>
</template>

<style scoped lang="scss"></style>

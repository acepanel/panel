<script setup lang="ts">
import { NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import api from '@/api/panel/file'
import { useFileStore, useTaskQueueStore } from '@/stores'
import { generateRandomString, lastDirectory } from '@/utils'

const { $gettext } = useGettext()
const fileStore = useFileStore()
const taskQueue = useTaskQueueStore()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const path = defineModel<string>('path', { type: String, required: true })
// 打开时快照选中项，弹窗内移除不影响列表选中状态
const paths = ref<string[]>([])
// 不含扩展名，扩展名由格式决定
const file = ref('')
const format = ref('.zip')

// 搜索子目录时可能是多级相对路径
const relative = (item: string) => item.slice(path.value.length).replace(/^\//, '')

// 单选以该项命名，多选以当前目录命名，根目录下避免出现 //xxx
const generateName = () => {
  const suffix = generateRandomString(6)
  if (paths.value.length === 1) return `${paths.value[0]}-${suffix}`
  if (path.value === '/') return `/${suffix}`
  return `${path.value}/${lastDirectory(path.value)}-${suffix}`
}

const handleArchive = async () => {
  const data = await api.compress(path.value, paths.value.map(relative), file.value + format.value)
  show.value = false
  fileStore.selected[fileStore.activeTabId] = []
  taskQueue.add(data)
}

// gzip 仅支持压缩单个文件
const formatOptions = computed(() => [
  { label: '.zip', value: '.zip' },
  { label: '.gz', value: '.gz', disabled: paths.value.length > 1 },
  { label: '.tar', value: '.tar' },
  { label: '.tar.gz', value: '.tar.gz' },
  { label: '.tgz', value: '.tgz' },
  { label: '.tar.bz2', value: '.tar.bz2' },
  { label: '.tar.xz', value: '.tar.xz' },
  { label: '.tar.zst', value: '.tar.zst' },
  { label: '.7z', value: '.7z' },
])

// 格式沿用上次的选择，多选时需避开 .gz
watch(show, (val) => {
  if (!val) return
  paths.value = [...fileStore.activeSelected]
  file.value = generateName()
  if (paths.value.length > 1 && format.value === '.gz') {
    format.value = '.zip'
  }
})
</script>

<template>
  <app-modal v-model:show="show" :title="$gettext('Compress')">
    <n-flex vertical>
      <n-form>
        <n-form-item>
          <template #label>
            {{ $gettext('Files to compress') }}
            <n-text depth="3">({{ paths.length }})</n-text>
          </template>
          <n-card content-style="padding: 8px" class="max-h-32 overflow-y-auto">
            <n-flex :size="8" class="path-tags">
              <n-tag
                v-for="item in paths"
                :key="item"
                :title="item"
                :closable="paths.length > 1"
                :bordered="false"
                class="max-w-full"
                @close="paths = paths.filter((p) => p !== item)"
              >
                {{ relative(item) }}
              </n-tag>
            </n-flex>
          </n-card>
        </n-form-item>
        <n-form-item :label="$gettext('Compress to')">
          <n-input-group>
            <n-input v-model:value="file" class="flex-1" />
            <n-select v-model:value="format" :options="formatOptions" class="w-24" />
          </n-input-group>
        </n-form-item>
      </n-form>
      <app-button type="primary" @click="handleArchive">
        {{ $gettext('Compress') }}
      </app-button>
    </n-flex>
  </app-modal>
</template>

<style scoped lang="scss">
.path-tags :deep(.n-tag__content) {
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>

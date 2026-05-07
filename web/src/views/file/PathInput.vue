<script setup lang="ts">
import copy2clipboard from '@vavt/copy2clipboard'
import type { InputInst } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import { useFileStore } from '@/stores'
import { checkPath } from '@/utils/file'

const { $gettext } = useGettext()
const fileStore = useFileStore()

const props = defineProps<{
  tabId: string
}>()

const tab = computed(() => fileStore.tabs.find((t) => t.id === props.tabId)!)
const path = computed(() => tab.value.path)

const isInput = ref(false)
const pathInput = ref<InputInst | null>(null)
const input = ref('www')

const handleInput = () => {
  isInput.value = true
  nextTick(() => {
    pathInput.value?.focus()
  })
}

// 双击地址栏复制路径
const handlePathDoubleClick = async () => {
  try {
    await copy2clipboard(path.value)
    window.$message.success($gettext('Path copied to clipboard'))
  } catch (error) {
    window.$message.error($gettext('Failed to copy path'))
  }
}

const handleBlur = () => {
  input.value = input.value.replace(/(^\/)|(\/$)/g, '')
  if (!checkPath(input.value)) {
    window.$message.error($gettext('Invalid path'))
    return
  }

  isInput.value = false
  fileStore.updateTabPath(props.tabId, '/' + input.value)
}

const handleRefresh = () => {
  window.$bus.emit('file:refresh')
}

// 切换显示隐藏文件
const toggleHidden = () => {
  fileStore.toggleShowHidden()
}

const handleUp = () => {
  const count = splitPath(path.value, '/').length
  setPath(count - 2)
}

const handleBack = () => {
  fileStore.historyBack(props.tabId)
}

const handleForward = () => {
  fileStore.historyForward(props.tabId)
}

const splitPath = (str: string, delimiter: string) => {
  if (str === delimiter || str === '') {
    return []
  }
  return str.split(delimiter).slice(1)
}

const setPath = (index: number) => {
  const newPath = splitPath(path.value, '/')
    .slice(0, index + 1)
    .join('/')
  fileStore.updateTabPath(props.tabId, '/' + newPath)
}

const handleSearch = () => {
  window.$bus.emit('file:search')
}

watch(
  path,
  (value) => {
    input.value = value.slice(1)
  },
  { immediate: true },
)
</script>

<template>
  <n-flex>
    <n-button-group>
      <n-tooltip>
        <template #trigger>
          <n-button @click="handleBack">
            <i-mdi-arrow-left class="text-base" />
          </n-button>
        </template>
        {{ $gettext('Back') }}
      </n-tooltip>
      <n-tooltip>
        <template #trigger>
          <n-button @click="handleForward">
            <i-mdi-arrow-right class="text-base" />
          </n-button>
        </template>
        {{ $gettext('Forward') }}
      </n-tooltip>
      <n-tooltip>
        <template #trigger>
          <n-button @click="handleUp">
            <i-mdi-arrow-up class="text-base" />
          </n-button>
        </template>
        {{ $gettext('Up') }}
      </n-tooltip>
      <n-tooltip>
        <template #trigger>
          <n-button @click="handleRefresh">
            <i-mdi-refresh class="text-base" />
          </n-button>
        </template>
        {{ $gettext('Refresh') }}
      </n-tooltip>
      <n-tooltip>
        <template #trigger>
          <n-button @click="toggleHidden" :type="fileStore.showHidden ? 'primary' : 'default'">
            <i-mdi-eye v-if="fileStore.showHidden" class="text-base" />
            <i-mdi-eye-off v-else class="text-base" />
          </n-button>
        </template>
        {{ fileStore.showHidden ? $gettext('Hide hidden files') : $gettext('Show hidden files') }}
      </n-tooltip>
    </n-button-group>
    <n-input-group flex-1>
      <n-tag
        size="large"
        v-if="!isInput"
        flex-1
        @click="handleInput"
        @dblclick="handlePathDoubleClick"
      >
        <n-breadcrumb separator=">">
          <n-breadcrumb-item @click.stop="setPath(-1)">
            {{ $gettext('Root Directory') }}
          </n-breadcrumb-item>
          <n-breadcrumb-item
            v-for="(item, index) in splitPath(path, '/')"
            :key="index"
            @click.stop="setPath(index)"
          >
            {{ item }}
          </n-breadcrumb-item>
        </n-breadcrumb>
      </n-tag>
      <n-input-group-label v-if="isInput">/</n-input-group-label>
      <n-input
        ref="pathInput"
        v-model:value="input"
        v-if="isInput"
        @keyup.enter="handleBlur"
        @blur="handleBlur"
      />
    </n-input-group>
    <n-input-group w-100>
      <n-input v-model:value="tab.keyword" :placeholder="$gettext('Enter search content')">
        <template #suffix>
          <n-checkbox v-model:checked="tab.sub">
            {{ $gettext('Include subdirectories') }}
          </n-checkbox>
        </template>
      </n-input>
      <n-button type="primary" @click="handleSearch">
        <i-mdi-search class="text-base" />
      </n-button>
    </n-input-group>
  </n-flex>
</template>

<style scoped lang="scss"></style>

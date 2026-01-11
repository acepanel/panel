<script setup lang="ts">
import type { InputInst } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import { useFileStore } from '@/store'
import { checkPath } from '@/utils/file'

const { $gettext } = useGettext()
const fileStore = useFileStore()
const path = defineModel<string>('path', { type: String, required: true }) // 当前路径
const keyword = defineModel<string>('keyword', { type: String, default: '' }) // 搜索关键词
const sub = defineModel<boolean>('sub', { type: Boolean, default: false }) // 搜索是否包括子目录
const isInput = ref(false)
const pathInput = ref<InputInst | null>(null)
const input = ref('www')

const history: string[] = []
let current = -1

const handleInput = () => {
  isInput.value = true
  nextTick(() => {
    pathInput.value?.focus()
  })
}

const handleBlur = () => {
  input.value = input.value.replace(/(^\/)|(\/$)/g, '')
  if (!checkPath(input.value)) {
    window.$message.error($gettext('Invalid path'))
    return
  }

  isInput.value = false
  path.value = '/' + input.value
  handlePushHistory(path.value)
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
  if (current > 0) {
    current--
    path.value = history[current]
    input.value = path.value.slice(1)
  }
}

const handleForward = () => {
  if (current < history.length - 1) {
    current++
    path.value = history[current]
    input.value = path.value.slice(1)
  }
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
  path.value = '/' + newPath
  input.value = newPath
  handlePushHistory(path.value)
}

const handlePushHistory = (path: string) => {
  // 防止在前进后退时重复添加
  if (current != history.length - 1) {
    return
  }

  history.splice(current + 1)
  history.push(path)
  current = history.length - 1
}

const handleSearch = () => {
  window.$bus.emit('file:search')
}

watch(
  path,
  (value) => {
    input.value = value.slice(1)
  },
  { immediate: true }
)

onMounted(() => {
  window.$bus.on('file:push-history', handlePushHistory)
})

onUnmounted(() => {
  window.$bus.off('file:push-history')
})
</script>

<template>
  <n-flex>
    <n-button-group>
      <n-button @click="handleBack">
        <i-mdi-arrow-left :size="16" />
      </n-button>
      <n-button @click="handleForward">
        <i-mdi-arrow-right :size="16" />
      </n-button>
      <n-button @click="handleUp">
        <i-mdi-arrow-up :size="16" />
      </n-button>
      <n-button @click="handleRefresh">
        <i-mdi-refresh :size="16" />
      </n-button>
      <n-tooltip>
        <template #trigger>
          <n-button @click="toggleHidden" :type="fileStore.showHidden ? 'primary' : 'default'">
            <i-mdi-eye v-if="fileStore.showHidden" :size="16" />
            <i-mdi-eye-off v-else :size="16" />
          </n-button>
        </template>
        {{ fileStore.showHidden ? $gettext('Hide hidden files') : $gettext('Show hidden files') }}
      </n-tooltip>
    </n-button-group>
    <n-input-group flex-1>
      <n-tag size="large" v-if="!isInput" flex-1 @click="handleInput">
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
    <n-input-group w-400>
      <n-input v-model:value="keyword" :placeholder="$gettext('Enter search content')">
        <template #suffix>
          <n-checkbox v-model:checked="sub">
            {{ $gettext('Include subdirectories') }}
          </n-checkbox>
        </template>
      </n-input>
      <n-button type="primary" @click="handleSearch">
        <i-mdi-search :size="16" />
      </n-button>
    </n-input-group>
  </n-flex>
</template>

<style scoped lang="scss"></style>

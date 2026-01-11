<script setup lang="ts">
defineOptions({
  name: 'file-index'
})

import { useFileStore } from '@/store'
import CompressModal from '@/views/file/CompressModal.vue'
import GridView from '@/views/file/GridView.vue'
import ListTable from '@/views/file/ListTable.vue'
import PathInput from '@/views/file/PathInput.vue'
import PermissionModal from '@/views/file/PermissionModal.vue'
import ToolBar from '@/views/file/ToolBar.vue'
import type { FileInfo, Marked } from '@/views/file/types'

const fileStore = useFileStore()

const selected = ref<string[]>([])
const marked = ref<Marked[]>([])
const markedType = ref<string>('copy')
// 权限编辑时的文件信息列表
const permissionFileInfoList = ref<FileInfo[]>([])

const compress = ref(false)
const permission = ref(false)
</script>

<template>
  <common-page show-footer>
    <n-flex vertical :size="20">
      <path-input
        v-model:path="fileStore.path"
        v-model:keyword="fileStore.keyword"
        v-model:sub="fileStore.sub"
      />
      <tool-bar
        v-model:path="fileStore.path"
        v-model:selected="selected"
        v-model:marked="marked"
        v-model:markedType="markedType"
        v-model:compress="compress"
        v-model:permission="permission"
        v-model:permission-file-info-list="permissionFileInfoList"
      />
      <!-- 根据视图类型切换显示 -->
      <list-table
        v-if="fileStore.viewType === 'list'"
        v-model:path="fileStore.path"
        v-model:keyword="fileStore.keyword"
        v-model:sub="fileStore.sub"
        v-model:selected="selected"
        v-model:marked="marked"
        v-model:markedType="markedType"
        v-model:compress="compress"
        v-model:permission="permission"
        v-model:permission-file-info-list="permissionFileInfoList"
      />
      <grid-view
        v-else
        v-model:path="fileStore.path"
        v-model:keyword="fileStore.keyword"
        v-model:sub="fileStore.sub"
        v-model:selected="selected"
        v-model:marked="marked"
        v-model:markedType="markedType"
        v-model:compress="compress"
        v-model:permission="permission"
        v-model:permission-file-info-list="permissionFileInfoList"
      />
      <compress-modal
        v-model:show="compress"
        v-model:path="fileStore.path"
        v-model:selected="selected"
      />
      <permission-modal
        v-model:show="permission"
        v-model:selected="selected"
        v-model:file-info-list="permissionFileInfoList"
      />
    </n-flex>
  </common-page>
</template>

<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const path = defineModel<string>('path', { type: String, required: true })

const mime = ref('')
const content = ref('')
const img = computed(() => {
  return `data:${mime.value};base64,${content.value}`
})

// 弹窗打开时加载，保证同一文件再次预览也能拿到最新内容
watch(show, (val) => {
  if (!val) return
  content.value = ''
  useRequest(file.content(path.value)).onSuccess(({ data }) => {
    mime.value = data.mime
    content.value = data.content
  })
})
</script>

<template>
  <app-modal v-model:show="show" :title="$gettext('Preview - ') + path">
    <n-image width="100%" :src="img" preview-disabled :show-toolbar="false">
      <template #placeholder>
        <n-spin />
      </template>
    </n-image>
  </app-modal>
</template>

<style scoped lang="scss"></style>
